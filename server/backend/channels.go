package backend

import (
	"fmt"
	"time"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/model"
	"github.com/patrickmn/go-cache"
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
		Id:          channel.Id,
		DisplayName: channel.DisplayName,
		Type:        string(channel.Type),
		TeamId:      channel.TeamId,
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

func (mm *MattermostBackend) GetChannelPosts(channelID string, fromt int64, tot int64) ([]*model.Post, error) {
	cacheKey := fmt.Sprintf("%s_%d_%d", channelID, fromt, tot)

	if x, found := mm.postsCache.Get(cacheKey); found {
		mm.LogDebug("Cache HIT for posts with cacheKey=%s", cacheKey)
		return x.([]*model.Post), nil
	}

	mm.LogDebug("Cache MISS for posts with cacheKey=%s", cacheKey)

	posts, err := mm.api.GetPostsSince(channelID, fromt)
	if err != nil {
		return nil, fmt.Errorf("error getting posts from db: %s", err)
	}

	res := []*model.Post{}

	for v := 0; v < len(posts.Order); v++ {
		post := posts.Posts[posts.Order[v]]

		// discard posts out of the range, deleted or of type different than normal (e.g., system messages)
		if post.CreateAt > tot || post.DeleteAt > 0 || post.Type != "" {
			continue
		}

		// elements  come from the API ordered from the newer to the older,
		// we sort elements in the reverse order... from the older to the newest
		newpost := &model.Post{
			Id:        post.Id,
			Type:      post.Type,
			Message:   post.Message,
			AuthorId:  post.UserId,
			CreatedAt: time.UnixMilli(post.CreateAt),
			RootId:    post.RootId,
		}
		res = append([]*model.Post{newpost}, res...)
	}

	mm.postsCache.Set(cacheKey, res, cache.DefaultExpiration)

	return res, nil
}

func (mm *MattermostBackend) GetChannelMembersForUser(teamID string, userID string) ([]*model.ChannelMembership, error) {
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
		if ch.TeamId != teamID {
			continue
		}

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
