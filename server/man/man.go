package man

import (
	"fmt"
	"sort"
	"time"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/backend"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/model"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/userstatus"
	"github.com/pkg/errors"
)

type MissedActivityOptions struct {
	LowerBound            time.Time
	LastNotifiedTimestamp time.Time
	UpperBound            time.Time
}

type MissedActivityNotifier struct {
	backend      *backend.MattermostBackend
	UserStatuses *userstatus.UserStatusTracker
	options      *MissedActivityOptions
}

func (man *MissedActivityNotifier) logDebug(message string, a ...any) {
	man.backend.LogDebug(fmt.Sprintf(message, a...))
}

func (man *MissedActivityNotifier) ProcessMessageValidForNotification(post *model.Post, conv *model.UnreadConversation, user *model.User, cma *model.ChannelMissedActivity) bool {
	if post.AuthorId == user.Id {
		cma.AppendLog("Removing post \"%s\" because the user is the author", post.Message)
		return false
	}

	// at the moment this check is done in the backend and so it is useless here
	// if post.Type != "" {
	//  	cma.AppendLog("Removing post \"%s\" because it is a system message", post.Message)
	//  	return false
	// }

	if !post.IsRoot() && !conv.Following && !user.MANPreferences.NotifyRepliesInNotFollowedThreads {
		if user.MANPreferences.IncludeCountOfRepliesInNotFollowedThreads {
			cma.RepliesInNotFollowingConvs++
		}
		cma.AppendLog("Removing post \"%s\" because it is a reply in a not followed thread", post.Message)
		return false
	}

	if (model.MessageContainsMentions(post.Message, user.Username) || cma.Channel.IsDirect() || conv.Following) && man.UserStatuses.GetStatusForUserAtTime(user.Id, post.CreatedAt) != userstatus.Online {
		if user.MANPreferences.InlcudeCountOfMessagesNotifiedByMM {
			cma.NotifiedByMMMessages++
		}
		cma.AppendLog("Removing post \"%s\" (created at: %d) because the user should have been already notified", post.Message, post.CreatedAt.UnixMilli())
		return false
	}

	if !post.CreatedAt.After(man.options.LastNotifiedTimestamp) {
		cma.AppendLog("Removing post \"%s\" because it is older than the last notified timestamp (so, it has been already notified)", post.Message)
		if user.MANPreferences.IncludeCountPreviouslyNotified {
			cma.PreviouslyNotified++
		}
		return false
	}

	return true
}

func (man *MissedActivityNotifier) GetChannelMissedActivity(channelMembership *model.ChannelMembership) (*model.ChannelMissedActivity, error) {
	// 1. Get all the posts in the channel that are unread for the user
	//  (up to the run upper bound)

	lowerBound := channelMembership.LastReadPost.UnixMilli()
	if man.options.LowerBound.UnixMilli() > lowerBound {
		lowerBound = man.options.LowerBound.UnixMilli()
	}

	posts, err := man.backend.GetChannelPosts(
		channelMembership.Channel.Id,
		lowerBound,
		man.options.UpperBound.UnixMilli())

	if err != nil {
		return nil, errors.Wrap(err, "Error getting channel posts")
	}

	if len(posts) == 0 {
		return nil, nil
	}

	crs := model.NewChannelMissedActivity(channelMembership.Channel, channelMembership.User)

	// 2. organize posts in conversations ***
	rootPostsMap := make(map[string]*model.UnreadConversation)

	for v := 0; v < len(posts); v++ {
		post := posts[v]

		if post.IsRoot() { // creates a new unread conversation for each root post
			rootPostsMap[post.Id] = model.NewUnreadConversation(
				post,
				man.backend.IsUserFollowingPost(post.Id, channelMembership.User.Id),
				channelMembership.LastReadPost.Before(post.CreatedAt),
			)
		} else { // add replies to conversations
			conversation := rootPostsMap[post.RootId]

			valid := man.ProcessMessageValidForNotification(post, conversation, channelMembership.User, crs)
			if valid {
				conversation.AppendReply(post)
			}
		}
	}

	// 3. filter conversations
	for _, conversation := range rootPostsMap {
		eligible := man.ProcessMessageValidForNotification(conversation.RootPost, conversation, channelMembership.User, crs)
		hasReplies := len(conversation.Replies) > 0
		isRootRead := !conversation.IsRootMessageUnread

		if (eligible && !isRootRead) || hasReplies {
			crs.UnreadConversations = append(crs.UnreadConversations, conversation)
		}
	}

	// 4. sort conversations
	sort.Slice(crs.UnreadConversations, func(i, j int) bool {
		return crs.UnreadConversations[i].MostRecentMessage.Before(crs.UnreadConversations[j].MostRecentMessage)
	})

	return crs, nil
}

func (man *MissedActivityNotifier) GetUserMissedActivity(team *model.Team, user *model.User, includeDirectMessages bool) (*model.TeamMissedActivity, error) {
	uma := &model.TeamMissedActivity{
		User:           user,
		Team:           team,
		UnreadChannels: []model.ChannelMissedActivity{},
		Logs:           []string{},
	}

	// 1. get channels memeberships
	mb, err := man.backend.GetChannelMembersForUser(team.Id, user.Id, includeDirectMessages)
	if err != nil {
		return nil, err
	}

	uchs := []model.ChannelMissedActivity{}

	// 2. for each not muted channel where the user is member, get the missed activity
	for _, channelMembership := range mb {
		if channelMembership.IsMuted() {
			man.logDebug("Skipping channel '%s' for user '%s' because it has been muted", channelMembership.Channel.GetChannelName(channelMembership.User), channelMembership.User.Username)
			continue
		}

		crs, errUCh := man.GetChannelMissedActivity(channelMembership)
		if errUCh != nil {
			return nil, errors.Wrap(errUCh, "Error computing user's missed activity")
		}

		if crs != nil {
			uchs = append(uchs, *crs)
		}
	}

	if len(uchs) == 0 {
		return nil, nil
	}
	uma.UnreadChannels = uchs
	return uma, nil
}

func (man *MissedActivityNotifier) Run() ([]*model.TeamMissedActivity, error) {
	// 1. get all users that are eligible to receive notifications
	//    (exclude system users and users that deactivated the plugin)
	users, err2 := man.backend.GetNotifiableUsers()
	if err2 != nil {
		return nil, errors.Wrap(err2, "Error getting user list, cannot continue")
	}

	res := []*model.TeamMissedActivity{}

	for _, user := range users {
		teams, err3 := man.backend.GetTeamsForUser(user.Id)
		if err3 != nil {
			return nil, errors.Wrap(err3, "Error getting team list, cannot continue")
		}

		// only one team. We include in this team also the direct messages
		if len(teams) == 1 {
			uma, err := man.GetUserMissedActivity(teams[0], user, true)

			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("error getting user missed activity in team %s (including direct messages), cannot continue", teams[0].Name))
			}

			if uma != nil {
				res = append(res, uma)
			}
		} else {
			// append a fake team to handle direct messages. Direct Messages does not belong
			// to a particular Team, so to manage them uniformely we use this special team
			teams = append(teams, model.DIRECT_MESSAGES_FAKE_TEAM)

			for _, team := range teams {
				uma, err := man.GetUserMissedActivity(team, user, false)

				if err != nil {
					return nil, errors.Wrap(err, fmt.Sprintf("error getting user missed activity in team %s, cannot continue", teams[0].Name))
				}

				if uma != nil {
					res = append(res, uma)
				}
			}
		}
	}
	return res, nil
}
