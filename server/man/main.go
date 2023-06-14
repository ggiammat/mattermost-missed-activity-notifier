package man

import (
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/backend"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/model"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/userstatus"
)

func RunMAN(backend *backend.MattermostBackend, userStatuses *userstatus.UserStatusTracker, options *MissedActivityOptions) ([]*model.TeamMissedActivity, error) {
	svc := &MissedActivityNotifier{
		backend:      backend,
		UserStatuses: userStatuses,
		options:      options,
	}

	return svc.Run()
}
