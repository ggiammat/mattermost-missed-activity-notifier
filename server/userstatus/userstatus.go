package userstatus

import (
	"errors"
	"time"
)

type UserStatus int8

const (
	Online UserStatus = iota
	Away
	Offline
	DND
	Custom
	Unknown
)

//nolint:revive
type UserStatusHistory struct {
	statuses   []UserStatus
	timestamps []int64
}

//nolint:revive
type UserStatusTracker struct {
	usersMap map[string]*UserStatusHistory
}

func (u *UserStatusTracker) GetTrackerUserIds() []string {
	keys := make([]string, len(u.usersMap))

	i := 0
	for k := range u.usersMap {
		keys[i] = k
		i++
	}
	return keys
}

func (u *UserStatusTracker) GetUserStatusHistory(userID string) ([]int64, []UserStatus) {
	return u.usersMap[userID].timestamps, u.usersMap[userID].statuses
}

func NewUserStatusesTracker() *UserStatusTracker {
	return &UserStatusTracker{usersMap: map[string]*UserStatusHistory{}}
}

func (u UserStatusTracker) cleanOlderThan(time time.Time) {
	for _, v := range u.usersMap {
		v.clearHistoyOlderThan(time)
	}
}

func (u UserStatusTracker) setStatusAtTime(userID string, status string, timestamp int64) error {
	encS := encodeStatus(status)

	if entry, ok := u.usersMap[userID]; ok {
		err := entry.SetStatusAt(encS, timestamp)
		u.usersMap[userID] = entry
		return err
	}

	entry := &UserStatusHistory{}
	err := entry.SetStatusAt(encS, timestamp)
	u.usersMap[userID] = entry
	return err
}

func (u UserStatusTracker) GetStatusForUserAtTime(userID string, time time.Time) UserStatus {
	if entry, ok := u.usersMap[userID]; ok {
		return entry.getStatusAt(time)
	}
	return Unknown
}

func encodeStatus(status string) UserStatus {
	var encodedStatus UserStatus

	switch status {
	case "online":
		encodedStatus = Online
	case "away":
		encodedStatus = Away
	case "offline":
		encodedStatus = Offline
	default:
		encodedStatus = Unknown
	}

	return encodedStatus
}

func (s *UserStatusHistory) SetStatusAt(newStatus UserStatus, timestamp int64) error {
	if len(s.timestamps) > 0 {
		if timestamp <= s.timestamps[len(s.timestamps)-1] {
			return errors.New("cannot set status for a time older than the last timestamp")
		}
		if s.statuses[len(s.statuses)-1] == newStatus {
			return nil
		}
	}

	s.statuses = append(s.statuses, newStatus)
	s.timestamps = append(s.timestamps, timestamp)

	return nil
}

func (s *UserStatusHistory) getStatusAt(time time.Time) UserStatus {
	t := time.UnixMilli()
	i := 0

	for i < len(s.timestamps) && s.timestamps[i] <= t {
		i++
	}

	if i == 0 {
		return Unknown
	}

	return s.statuses[i-1]
}

func (s *UserStatusHistory) clearHistoyOlderThan(time time.Time) {
	t := time.UnixMilli()
	i := 0

	for i < len(s.timestamps) && s.timestamps[i] < t {
		i++
	}

	if i == 0 {
		return
	}

	// if all entries are older than time, keep the last one
	// because it is the current status
	if i >= len(s.timestamps) {
		i = len(s.timestamps) - 1
	}

	s.statuses = s.statuses[i:]
	s.timestamps = s.timestamps[i:]
}
