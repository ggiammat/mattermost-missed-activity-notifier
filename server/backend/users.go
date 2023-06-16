package backend

import (
	"fmt"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/model"
	mm_model "github.com/mattermost/mattermost-server/v6/model"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
)

func (mm *MattermostBackend) getUsersInChannel(channelId string) ([]*model.User, error) {

	// TODO: handle pagination
	users, err := mm.api.GetUsersInChannel(channelId, "username", 0, 1000)

	if err != nil {
		return nil, fmt.Errorf("error getting users in channel (channelId=%s): %s", channelId, err)
	} else {
		members := []*model.User{}
		for k := 0; k < len(users); k++ {
			u, _ := mm.GetUser(users[k].Id)
			members = append(members, u)
		}
		return members, nil
	}
}

func (mm *MattermostBackend) SendEmailToUser(user *model.User, subject string, body string) error {
	err := mm.api.SendMail(user.Email, subject, body)
	if err != nil {
		return errors.Wrap(err, "error sending email")
	}
	return nil
}

func (mm *MattermostBackend) GetUser(userID string) (*model.User, error) {
	if x, found := mm.usersCache.Get(userID); found {
		mm.LogDebug("Cache HIT for userId=%s", userID)
		return x.(*model.User), nil
	}

	mm.LogDebug("Cache MISS for userId=%s", userID)

	users, err := mm.loadUsers(userID)
	if err != nil {
		return nil, fmt.Errorf("error getting user from db: %s", err)
	}

	if len(users) > 0 {
		return users[0], nil
	}

	return nil, fmt.Errorf("user not found (userId=%s)", userID)
}

func (mm *MattermostBackend) IsUserFollowingPost(postID string, userID string) bool {
	rows, err := mm.db.Query("SELECT Following FROM ThreadMemberships WHERE PostId = ? AND UserId = ?", postID, userID)
	if err != nil {
		mm.api.LogError(fmt.Sprintf("Error querying db for threads following: %s", err))
	}
	defer rows.Close()

	if rows.Next() { // assuming exactly one result
		var following int8
		if err2 := rows.Scan(&following); err2 != nil {
			mm.api.LogError(fmt.Sprintf("Error scanning rows: %s", err2))
		}
		return following > 0
	}
	return false
}

func (mm *MattermostBackend) GetNotifiableUsers() ([]*model.User, error) {
	users, err := mm.loadUsers("")
	if err != nil {
		return nil, fmt.Errorf("error getting teams list, cannot continue: %s", err)
	}

	emailVerificationEnabled := mm.IsEmailVerificationEnabled()

	res := []*model.User{}

	// filter users
	for _, u := range users {
		if u.IsBot ||
			!u.MANPreferences.Enabled ||
			(!u.EmailVerified && emailVerificationEnabled) ||
			!u.EmailsEnabled {
			continue
		}
		res = append(res, u)
	}

	return res, nil
}

/*
Called by the status tracker to get the status of all users. It is lighter than
calling loadUsers
*/
func (mm *MattermostBackend) GetUsersStatus() (map[string]string, error) {
	// we store all statuses in a single key
	if x, found := mm.userStatusCache.Get("__allusersstatus"); found {
		mm.LogDebug("Cache HIT for userstatuses")
		return x.(map[string]string), nil
	}

	mm.LogDebug("Cache MISS for userstatuses")

	// 1. get ids of all existing users
	ids := []string{}
	rows, err := mm.db.Query("select Id from Users;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, errors.Wrap(err, "Error reading query results")
		}
		ids = append(ids, id)
	}

	mmStatuses, errS := mm.api.GetUserStatusesByIds(ids)
	if errS != nil {
		return nil, errors.Wrap(errS, "Error getting users' status from Mattermost API")
	}
	userStatuses := map[string]string{}
	for _, ms := range mmStatuses {
		userStatuses[ms.UserId] = ms.Status
	}

	mm.userStatusCache.Set("__allusersstatus", userStatuses, cache.DefaultExpiration)

	return userStatuses, nil
}

func (mm *MattermostBackend) loadUsers(userID string) ([]*model.User, error) {
	var mmUsers []*mm_model.User

	// 1. Get users from Mattermost API

	if userID != "" { // search a single user
		singleUser, errU := mm.api.GetUser(userID)
		if errU != nil {
			return nil, errors.Wrap(errU, "Error getting users from Mattermost API")
		}
		mmUsers = []*mm_model.User{singleUser}
	} else { // search all users
		// TODO: handle pagination
		allUsers, errU := mm.api.GetUsers(&mm_model.UserGetOptions{Page: 0, PerPage: 10000})
		if errU != nil {
			return nil, errors.Wrap(errU, "Error getting users from Mattermost API")
		}
		mmUsers = allUsers
	}

	// 2. Get status from Mattermost API

	userStatuses, errS := mm.GetUsersStatus()
	if errS != nil {
		return nil, errors.Wrap(errS, "Error getting users' status from Mattermost API")
	}

	// 3. Build user objects

	var res []*model.User
	for _, u := range mmUsers {

		newU := &model.User{
			Id:            u.Id,
			Username:      u.Username,
			Email:         u.Email,
			FirstName:     u.FirstName,
			LastName:      u.LastName,
			Active:        u.DeleteAt == 0,
			EmailVerified: u.EmailVerified,
			EmailsEnabled: u.NotifyProps["email"] != "false",
			IsBot:         u.IsBot,
			Roles:         u.GetRoles(),
			Status:        userStatuses[u.Id],
		}

		// add profile image
		profileImage, errI := mm.api.GetProfileImage(newU.Id)
		if errI != nil {
			mm.LogError("Error getting profile image for user %s: %s", newU.Username, errI)
		} else {
			newU.Image = profileImage
		}

		// load MAN preferences
		newU.MANPreferences = mm.GetPreferencesForUser(newU.Id)

		// add object to cache
		mm.usersCache.Set(newU.Id, newU, cache.DefaultExpiration)

		res = append(res, newU)
	}

	return res, nil
}
