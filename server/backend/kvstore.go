package backend

import (
	"encoding/json"
	"time"

	"github.com/ggiammat/missed-activity-notifications/server/model"
	"github.com/pkg/errors"
)

type MANKVStore struct {
	UserPreferences       map[string]model.MANUserPreferences
	LastNotifiedTimestamp int64
}

func (mm *MattermostBackend) GetLastNotifiedTimestamp() (time.Time, error) {
	store, err := mm.getKVStore()

	if err != nil {
		return time.Time{}, errors.Wrap(err, "Error getting kvStore in GetLastNotifiedTimestamp")
	}

	return time.UnixMilli(store.LastNotifiedTimestamp), nil
}

func (mm *MattermostBackend) SetLastNotifiedTimestamp(value time.Time) error {
	store, err := mm.getKVStore()

	if err != nil {
		return errors.Wrap(err, "Error getting kvStore while setting last notified timetamp")
	}

	store.LastNotifiedTimestamp = value.UnixMilli()

	err2 := mm.saveKV()
	if err2 != nil {
		return errors.Wrap(err2, "Error saving kvStore while setting last notified timetamp")
	}

	return nil
}

func (mm *MattermostBackend) ResetPreferenceEnabled(user *model.User) error {
	pref, err := mm.getKVStore()

	if err != nil {
		return errors.Wrap(err, "Error getting kvStore to set user preference")
	}

	delete(pref.UserPreferences, user.Id)

	mm.saveKV()
	mm.usersCache.Delete(user.Id)

	return nil
}

func (mm *MattermostBackend) GetPreferencesForUser(userID string) model.MANUserPreferences {
	// load MAN preferences
	prefs, kvErr := mm.getKVStore()
	if kvErr != nil {
		mm.LogError("error loading MAN Preferences for user %s: %s", userID, kvErr)
	}

	if entry, ok := prefs.UserPreferences[userID]; ok {
		return entry
	} else {
		defaultCopy := *mm.defaultUserPrefs
		return defaultCopy
	}
}

func (mm *MattermostBackend) SetPreferencesForUser(userID string, prefs model.MANUserPreferences) error {
	// load MAN preferences
	store, kvErr := mm.getKVStore()
	if kvErr != nil {
		mm.LogError("error loading MAN Preferences for user %s: %s", userID, kvErr)
	}

	store.UserPreferences[userID] = prefs
	mm.saveKV()
	mm.usersCache.Delete(userID)
	return nil
}

func (mm *MattermostBackend) SetPreferenceNotifyRepliesNotFollowed(user *model.User, enabled bool) error {
	prefs := mm.GetPreferencesForUser(user.Id)

	if prefs.NotifyRepliesInNotFollowedThreads != enabled {
		prefs.NotifyRepliesInNotFollowedThreads = enabled
		err := mm.SetPreferencesForUser(user.Id, prefs)
		if err != nil {
			return errors.Wrap(err, "error setting preferences for user")
		}
	}

	return nil
}

func (mm *MattermostBackend) SetPrefCountPreviouslyNotified(user *model.User, enabled bool) error {
	prefs := mm.GetPreferencesForUser(user.Id)

	if prefs.IncludeCountPreviouslyNotified != enabled {
		prefs.IncludeCountPreviouslyNotified = enabled
		err := mm.SetPreferencesForUser(user.Id, prefs)
		if err != nil {
			return errors.Wrap(err, "error setting preferences for user")
		}
	}

	return nil
}

func (mm *MattermostBackend) SetPreferenceCountNotifiedByMM(user *model.User, enabled bool) error {
	prefs := mm.GetPreferencesForUser(user.Id)

	if prefs.InlcudeCountOfMessagesNotifiedByMM != enabled {
		prefs.InlcudeCountOfMessagesNotifiedByMM = enabled
		err := mm.SetPreferencesForUser(user.Id, prefs)
		if err != nil {
			return errors.Wrap(err, "error setting preferences for user")
		}
	}

	return nil
}

func (mm *MattermostBackend) SetPreferenceCountRepliesNotFollowed(user *model.User, enabled bool) error {
	prefs := mm.GetPreferencesForUser(user.Id)

	if prefs.IncludeCountOfRepliesInNotFollowedThreads != enabled {
		prefs.IncludeCountOfRepliesInNotFollowedThreads = enabled
		err := mm.SetPreferencesForUser(user.Id, prefs)
		if err != nil {
			return errors.Wrap(err, "error setting preferences for user")
		}
	}

	return nil
}

func (mm *MattermostBackend) SetPreferenceEnabled(user *model.User, enabled bool) error {
	prefs := mm.GetPreferencesForUser(user.Id)
	if prefs.Enabled != enabled {
		prefs.Enabled = enabled
		err := mm.SetPreferencesForUser(user.Id, prefs)
		if err != nil {
			return errors.Wrap(err, "error setting preferences for user")
		}
	}

	return nil
}

func (mm *MattermostBackend) saveKV() error {
	ser, errSer := json.Marshal(mm.kvStoreCache)

	if errSer != nil {
		return errors.Wrap(errSer, "Error serializing kvStore")
	}

	errSet := mm.api.KVSet("kvstore", ser)
	if errSet != nil {
		return errors.Wrap(errSet, "Error saving kvStore")
	}

	return nil
}

func (mm *MattermostBackend) getKVStore() (*MANKVStore, error) {
	if mm.kvStoreCache != nil {
		// mm.api.LogDebug("Cache HIT for kvStore")
		return mm.kvStoreCache, nil
	}

	bytes, bErr := mm.api.KVGet("kvstore")
	if bErr != nil {
		return nil, errors.Wrap(bErr, "Error getting kvStore")
	}

	// key does not exist
	if bytes == nil {
		mm.kvStoreCache = &MANKVStore{
			UserPreferences:       map[string]model.MANUserPreferences{},
			LastNotifiedTimestamp: 0,
		}
		mm.saveKV()
		return mm.kvStoreCache, nil
	}

	var store MANKVStore
	if err := json.Unmarshal(bytes, &store); err != nil {
		return nil, errors.Wrap(err, "Error unseralizing kvStore")
	}

	mm.kvStoreCache = &store

	return mm.kvStoreCache, nil
}
