package backend

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	mm_model "github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/patrickmn/go-cache"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/model"
)

type MattermostBackend struct {
	api           plugin.API
	db            *sql.DB
	allowDebugLog bool

	defaultUserPrefs *model.MANUserPreferences

	// cache users, posts and channels since during a run of MAN Plugin,
	// same objects will be requested multiple times. These caches are
	// configured to expire before the next run, so each run will load
	// fresh data. The usersCache is loaded when loadUsers is invoked
	// (at the beginning of a MAN run to load the notifiableUsers)
	usersCache    *cache.Cache
	postsCache    *cache.Cache
	channelsCache *cache.Cache

	// object stored in the Mattermost kvstore. We keep a copy
	// in memory for reads (since we are the only ones to modify it)
	kvStoreCache *MANKVStore

	// 30 seconds cache for user statuses
	userStatusCache *cache.Cache
}

func CreateTeam(mmTeam *mm_model.Team) *model.Team {
	return &model.Team{
		ID:   mmTeam.Id,
		Name: mmTeam.DisplayName,
	}
}

func CreateTeamArray(array []*mm_model.Team) []*model.Team {
	res := make([]*model.Team, len(array))

	for i := 0; i < len(array); i++ {
		res[i] = CreateTeam(array[i])
	}

	return res
}

func NewMattermostBackend(api plugin.API, db *sql.DB, cacheExpiryTime int, enableDebugLog bool, defaultUserPrefs *model.MANUserPreferences) (*MattermostBackend, error) {
	svc := &MattermostBackend{
		api:              api,
		db:               db,
		allowDebugLog:    enableDebugLog,
		channelsCache:    cache.New(time.Duration(cacheExpiryTime)*time.Minute, 10*time.Minute),
		usersCache:       cache.New(time.Duration(cacheExpiryTime)*time.Minute, 10*time.Minute),
		postsCache:       cache.New(time.Duration(cacheExpiryTime)*time.Minute, 10*time.Minute),
		userStatusCache:  cache.New(30*time.Second, 1*time.Minute),
		defaultUserPrefs: defaultUserPrefs,
		kvStoreCache:     nil,
	}

	svc.LogInfo("New MattermostBackend initialized with cache expiry time %d min", cacheExpiryTime)

	return svc, nil
}

func (mm *MattermostBackend) LogInfo(message string, a ...any) {
	mm.api.LogInfo(fmt.Sprintf(message, a...))
}

func (mm *MattermostBackend) LogWarn(message string, a ...any) {
	mm.api.LogWarn(fmt.Sprintf(message, a...))
}

func (mm *MattermostBackend) LogDebug(message string, a ...any) {
	if !mm.allowDebugLog {
		return
	}
	mm.api.LogDebug(fmt.Sprintf(message, a...))
}

func (mm *MattermostBackend) LogError(message string, a ...any) {
	mm.api.LogError(fmt.Sprintf(message, a...))
}

func (mm *MattermostBackend) GetReadmeContent() string {
	pluginRoot, _ := mm.api.GetBundlePath()
	content, errF := os.ReadFile(filepath.Join(pluginRoot, "README.md"))
	if errF != nil {
		mm.LogError("Error reading README.md")
		return "Help not available (error reading README.md file)"
	}
	return string(content)
}

func (mm *MattermostBackend) GetTemplatesPath() string {
	pluginRoot, _ := mm.api.GetBundlePath()
	return filepath.Join(pluginRoot, "assets/templates")
}

func (mm *MattermostBackend) GetServerName() string {
	res := mm.api.GetConfig().TeamSettings.SiteName
	if res != nil {
		return *res
	}
	return "Mattermost"
}

func (mm *MattermostBackend) GetServerURL() string {
	res := mm.api.GetConfig().ServiceSettings.SiteURL
	if res != nil {
		return *res
	}
	return "http://localhost/"
}

func (mm *MattermostBackend) IsEmailVerificationEnabled() bool {
	value := mm.api.GetConfig().EmailSettings.RequireEmailVerification
	if value != nil {
		return *value
	}
	return false
}
