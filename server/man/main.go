package man

import (
	"github.com/ggiammat/missed-activity-notifications/server/backend"
	"github.com/ggiammat/missed-activity-notifications/server/model"
	"github.com/ggiammat/missed-activity-notifications/server/userstatus"
)

func RunMAN(backend *backend.MattermostBackend, userStatuses *userstatus.UserStatusTracker, options *MissedActivityOptions) ([]*model.TeamMissedActivity, error) {
	svc := &MissedActivityNotifier{
		backend:      backend,
		UserStatuses: userStatuses,
		options:      options,
	}

	return svc.Run()
}
