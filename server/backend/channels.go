package backend

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	mm_model "github.com/mattermost/mattermost/server/public/model"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/model"
)

func (mm *MattermostBackend) GetChannel(channelID string) (*model.Channel, error) {
	if x, found := mm.channelsCache.Get(channelID); found {
		mm.LogDebug("Cache HIT for channelId=%s", channelID)
		return x.(*model.Channel), nil
	}

	mm.LogDebug("Cache MISS for channelId=%s", channelID)

	channel, err := mm.api.GetChannel(channelID)
	if err != nil {
		return nil, fmt.Errorf("error getting channel from db: %s", err)
	}

	res := &model.Channel{
		ID:          channel.Id,
		DisplayName: channel.DisplayName,
		Type:        string(channel.Type),
		TeamID:      channel.TeamId,
	}

	// Direct and group channels do not have a name, so we get the members to build
	// names like "Chat with, x, y"
	if channel.Type == "D" || channel.Type == "G" {
		// TODO: handle pagination
		members, err2 := mm.api.GetUsersInChannel(channelID, "username", 0, 1000)
		if err2 != nil {
			mm.api.LogError(fmt.Sprintf("Error getting members of channel: %s", err2))
		}
		memberDisplayNames := make([]string, len(members))
		for i, m := range members {
			u, _ := mm.GetUser(m.Id)
			memberDisplayNames[i] = u.DisplayName()
		}
		res.Members = memberDisplayNames
	}

	mm.channelsCache.Set(channel.Id, res, cache.DefaultExpiration)

	return res, nil
}

// returns the list of posts in a channel between two given timestamp.
// The returned list of posts also includes root posts even of the
// requested range to let the caller be able to rebuild the threads.
// results are returned from the oldest to the newer with root posts at the beginning
func (mm *MattermostBackend) GetChannelPosts(channelID string, fromt int64, tot int64) ([]*model.Post, error) {
	cacheKey := fmt.Sprintf("%s_%d_%d", channelID, fromt, tot)

	if x, found := mm.postsCache.Get(cacheKey); found {
		mm.LogDebug("Cache HIT for posts with cacheKey=%s", cacheKey)
		return x.([]*model.Post), nil
	}

	mm.LogDebug("Cache MISS for posts with cacheKey=%s", cacheKey)

	apiPosts, err := mm.api.GetPostsSince(channelID, fromt)
	if err != nil {
		return nil, fmt.Errorf("error getting posts from db: %s", err)
	}

	// Ensure that all root posts are also loaded
	// usually the Mattermost GetPostsSince() api includes root posts even if
	// they are created before the requested timestamp. However we observed
	// some situations where this does not happen. For instance if a reply
	// has a reaction, then the root post of this reply is not returned (if
	// older than the requested timestamp)...
	rootPosts := []*mm_model.Post{}
	rootPostsIds := []string{}
	for _, p := range apiPosts.Posts {
		if p.RootId != "" && !slices.Contains(apiPosts.Order, p.RootId) && !slices.Contains(rootPostsIds, p.RootId) {
			mm.LogDebug("Loading root post for post %s", p.Id)
			rootPost, errRP := mm.api.GetPost(p.RootId)
			if errRP != nil {
				return nil, errors.Wrap(errRP, "error loading root post")
			}
			rootPosts = append(rootPosts, rootPost)
			rootPostsIds = append(rootPostsIds, rootPost.Id)
		}
	}

	allPosts := []*mm_model.Post{}
	allPosts = append(allPosts, rootPosts...)
	allPosts = append(allPosts, apiPosts.ToSlice()...)

	res := []*model.Post{}

	for _, post := range allPosts {
		// discard posts out of the range or deleted
		if post.CreateAt > tot || post.DeleteAt > 0 {
			mm.LogDebug("Discarding post '%s' (%s) createAt: %d, tot:  %d, deleteAT: %d", post.Message, post.Id, post.CreateAt, tot, post.DeleteAt)
			continue
		}

		postProps := post.GetProps()

		fromBot := false
		if val, ok := postProps["from_bot"]; ok {
			res, err := strconv.ParseBool(val.(string))
			if err != nil {
				mm.LogError("error parsing 'from_bot' property: %+v", val)
			} else {
				fromBot = res
			}
		}

		msg := post.Message
		// in some cases (e.g., messages from boards bot) does not have the text in the Message field, but it is in the props
		// this is an hack to get the text of the message. The type conversions could be avoided using Mattermost's types like
		// PostTypeSlackAttachment
		if msg == "" {
			if val, ok := postProps["attachments"]; ok {
				x := val.([]interface{})[0].(map[string]interface{})
				msg = x["fallback"].(string)
				// hack to avoid having message interpreted as heading
				if strings.HasPrefix(msg, "######") {
					msg = msg[7:]
				}
			}
		}

		newpost := &model.Post{
			ID:              post.Id,
			Type:            post.Type,
			Message:         msg,
			AuthorID:        post.UserId,
			CreatedAt:       time.UnixMilli(post.CreateAt),
			RootID:          post.RootId,
			FromBot:         fromBot,
			IsSystemMessage: strings.HasPrefix(post.Type, "system_"),
		}
		res = append(res, newpost)
	}

	mm.postsCache.Set(cacheKey, res, cache.DefaultExpiration)

	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedAt.Before(res[j].CreatedAt)
	})

	return res, nil
}

func (mm *MattermostBackend) GetChannelMembersForUser(teamID string, userID string, includeDirectMessages bool) ([]*model.ChannelMembership, error) {
	// apparently the teamId parameter is not used, so this call return the memberships
	// of the user in ALL teams. So we remove all the results that are not in the teamId we
	// are looking for.
	// ref: https://github.com/mattermost/mattermost-server/blob/0cb3a406da7a339cc47bb72e32106b24e13c2a9a/server/channels/app/plugin_api.go#L593
	// TODO: handle pagination
	memberships, err := mm.api.GetChannelMembersForUser(teamID, userID, 0, 1000)

	if err != nil {
		return nil, fmt.Errorf("error getting channgel memberships from DB: %s", err)
	}

	res := []*model.ChannelMembership{}

	for _, mb := range memberships {
		ch, err := mm.GetChannel(mb.ChannelId)

		if err != nil {
			return nil, fmt.Errorf("error getting channel while getting channel memberships: %s", err)
		}

		// the api call GetChannelMembersForUser returns all memberships in ALL teams
		// so we remove the ones in other teams
		if ch.TeamID == teamID || (includeDirectMessages && ch.TeamID == "") {
			usr, err := mm.GetUser(mb.UserId)
			if err != nil {
				return nil, fmt.Errorf("error getting user while getting channel memberships: %s", err)
			}

			res = append(res, &model.ChannelMembership{
				Channel:      ch,
				User:         usr,
				LastReadPost: time.UnixMilli(mb.LastViewedAt),
				NotifyProps:  mb.NotifyProps,
			})
		}
	}

	return res, nil
}

func (mm *MattermostBackend) GetTeamsForUser(userID string) ([]*model.Team, error) {
	teams, err := mm.api.GetTeamsForUser(userID)

	if err != nil {
		return nil, fmt.Errorf("error getting teams for user from Mattermost API: %s", err)
	}

	return CreateTeamArray(teams), nil
}

func (mm *MattermostBackend) GetTeams() ([]*model.Team, error) {
	teams, err := mm.api.GetTeams()

	if err != nil {
		return nil, fmt.Errorf("error getting teams from Mattermost API: %s", err)
	}

	return CreateTeamArray(teams), nil
}
