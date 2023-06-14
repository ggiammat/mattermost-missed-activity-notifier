package userstatus

import (
	"fmt"
	"time"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/backend"
)

func TrackUserStatuses(statuses *UserStatusTracker, backend *backend.MattermostBackend, timestamp int64) {
	backend.LogDebug("Tracking user statuses")
	userStatuses, err := backend.GetUsersStatus()
	if err != nil {
		fmt.Printf("Error getting users for tracking user statuses: %s", err)
		return
	}

	for id, status := range userStatuses {
		err := statuses.setStatusAtTime(id, status, timestamp)
		if err != nil {
			backend.LogWarn("Error tracking user status for user %s: %s", id, err)
		}
	}
}

func ClearStatusesOlderThan(statuses *UserStatusTracker, time time.Time) {
	statuses.cleanOlderThan(time)
}
