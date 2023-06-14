package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	_ "net/http/pprof"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/backend"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/model"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/userstatus"
	"github.com/mattermost/mattermost-plugin-api/cluster"
	mm_model "github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/mattermost/mattermost-server/v6/shared/driver"
	"github.com/pkg/errors"
)

type MANPlugin struct {
	plugin.MattermostPlugin
	configurationLock sync.RWMutex
	configuration     *configuration
	userStatuses      *userstatus.UserStatusTracker
	manRunStats       *MANRunStats
	backend           *backend.MattermostBackend
	manJob            *cluster.Job
}

type MANRunLog struct {
	numRun        int
	from          time.Time
	to            time.Time
	executionTime time.Time
	textLogs      []string
	htmlEmails    []string
}

type MANRunStats struct {
	numRuns        int
	runLogs        []MANRunLog
	sentEmailStats map[string][]time.Time
}

func (p *MANPlugin) CreateMattermostBackend() error {

	cacheExpiryTime := math.Max(float64(p.configuration.RunInterval)/2, 0)

	defaultUserPref := &model.MANUserPreferences{
		Enabled:                           p.configuration.UserDefaultPrefEnabled,
		NotifyRepliesInNotFollowedThreads: p.configuration.UserDefaultPrefNotifyNotFollowed,
		IncludeCountOfRepliesInNotFollowedThreads: p.configuration.UserDefaultPrefCountNotFollowed,
		InlcudeCountOfMessagesNotifiedByMM:        p.configuration.UserDefaultPrefCountMM,
		IncludeCountPreviouslyNotified:            p.configuration.UserDefaultPrefCountPreviouslyNotified,
	}

	backend, err := backend.NewMattermostBackend(
		p.API,
		sql.OpenDB(driver.NewConnector(p.Driver, true)),
		int(cacheExpiryTime),
		p.configuration.DebugLogEnabled,
		defaultUserPref,
	)
	if err != nil {
		return err
	}
	p.backend = backend
	return nil
}

func (p *MANPlugin) OnActivate() error {

	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	// get user status now to populate statuses with an initial entry
	p.userStatuses = userstatus.NewUserStatusesTracker()
	userstatus.TrackUserStatuses(p.userStatuses, p.backend, time.Now().UnixMilli())

	if p.configuration.ResetLastNotificationTimestamp {
		errT := p.backend.SetLastNotifiedTimestamp(time.UnixMilli(0))
		if errT != nil {
			return errors.Wrap(errT, "error setting last notified timestamp")
		}
	}

	if p.configuration.NotifyOnlyNewMessagesFromStartup {
		errTT := p.backend.SetLastNotifiedTimestamp(time.Now())
		if errTT != nil {
			return errors.Wrap(errTT, "error setting last notified timestamp")
		}
	}

	p.manRunStats = &MANRunStats{
		numRuns:        0,
		runLogs:        []MANRunLog{},
		sentEmailStats: map[string][]time.Time{},
	}

	err3 := p.activateMANJob()
	if err3 != nil {
		return fmt.Errorf("error activating MANJob: %v", err3)
	}

	errC := p.registerMANCommand()
	if errC != nil {
		return errors.Wrap(errC, "error registering command")
	}

	return nil
}

func (p *MANPlugin) OnDeactivate() error {

	err := p.deactivateMANJob()
	if err != nil {
		return errors.Wrap(err, "Error deactivagin plugin")
	}

	return nil
}

func (p *MANPlugin) MessageHasBeenPosted(_ *plugin.Context, post *mm_model.Post) {
	userstatus.TrackUserStatuses(p.userStatuses, p.backend, post.CreateAt)
}
