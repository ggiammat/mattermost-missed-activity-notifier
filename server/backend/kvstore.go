package backend

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/model"
	"github.com/oleiade/reflections"
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

func (mm *MattermostBackend) ResetAllUserPrefernces() error {
	pref, err := mm.getKVStore()

	if err != nil {
		return errors.Wrap(err, "Error getting kvStore to set user preference")
	}

	pref.UserPreferences = map[string]model.MANUserPreferences{}

	errS := mm.saveKV()
	if errS != nil {
		return errors.Wrap(errS, "error saving kvstore")
	}

	for k, _ := range mm.usersCache.Items() {
		mm.usersCache.Delete(k)
	}
	return nil
}

func (mm *MattermostBackend) ResetPreferences(user *model.User) error {
	pref, err := mm.getKVStore()

	if err != nil {
		return errors.Wrap(err, "Error getting kvStore to set user preference")
	}

	delete(pref.UserPreferences, user.ID)

	errS := mm.saveKV()
	if errS != nil {
		return errors.Wrap(errS, "error saving kvstore")
	}
	mm.usersCache.Delete(user.ID)

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
	}

	defaultCopy := *mm.defaultUserPrefs
	return defaultCopy
}

func (mm *MattermostBackend) SetPreferencesForUser(userID string, prefs model.MANUserPreferences) error {
	// load MAN preferences
	store, kvErr := mm.getKVStore()
	if kvErr != nil {
		mm.LogError("error loading MAN Preferences for user %s: %s", userID, kvErr)
	}

	store.UserPreferences[userID] = prefs
	errS := mm.saveKV()
	if errS != nil {
		return errors.Wrap(errS, "error saving kvstore")
	}
	mm.usersCache.Delete(userID)
	return nil
}

func (mm *MattermostBackend) SetUserPreference(user *model.User, name string, newValue any) error {
	prefs := mm.GetPreferencesForUser(user.ID)

	has, _ := reflections.HasField(prefs, name)

	if has {
		currentValue, _ := reflections.GetField(prefs, name)
		if currentValue != newValue {
			mm.LogDebug("*********** field current value: %s, field type: %s", currentValue, reflect.TypeOf(currentValue))
			errF := reflections.SetField(&prefs, name, newValue)
			if errF != nil {
				return errors.Wrap(errF, "error setting preference value for user")
			}
			errS := mm.SetPreferencesForUser(user.ID, prefs)
			if errS != nil {
				return errors.Wrap(errS, "error savling preferences for user")
			}
			mm.LogDebug("*********** new value set for preference '%s' for user '%s': %s", name, user.Username, newValue)
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
		errS := mm.saveKV()
		if errS != nil {
			return nil, errors.Wrap(errS, "error saving kvstore")
		}
		return mm.kvStoreCache, nil
	}

	var store MANKVStore
	if err := json.Unmarshal(bytes, &store); err != nil {
		return nil, errors.Wrap(err, "Error unseralizing kvStore")
	}

	mm.kvStoreCache = &store

	return mm.kvStoreCache, nil
}
