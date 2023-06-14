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

type UserStatusHistory struct {
	statuses   []UserStatus
	timestamps []int64
}

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

func (u *UserStatusTracker) GetUserStatusHistory(userId string) ([]int64, []UserStatus) {
	return u.usersMap[userId].timestamps, u.usersMap[userId].statuses
}

func NewUserStatusesTracker() *UserStatusTracker {
	return &UserStatusTracker{usersMap: map[string]*UserStatusHistory{}}
}

func (a UserStatusTracker) cleanOlderThan(time time.Time) {
	for _, v := range a.usersMap {
		v.clearHistoyOlderThan(time)
	}
}

func (a UserStatusTracker) setStatusAtTime(userId string, status string, timestamp int64) error {
	encS := encodeStatus(status)
	if entry, ok := a.usersMap[userId]; ok {
		err := entry.SetStatusAt(encS, timestamp)
		a.usersMap[userId] = entry
		return err
	} else {
		entry := &UserStatusHistory{}
		err := entry.SetStatusAt(encS, timestamp)
		a.usersMap[userId] = entry
		return err
	}
}

func (a UserStatusTracker) GetStatusForUserAtTime(userId string, time time.Time) UserStatus {
	if entry, ok := a.usersMap[userId]; ok {
		return entry.getStatusAt(time)
	} else {
		return Unknown
	}
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

func (s *UserStatusHistory) clearHistory() {
	i := len(s.statuses) - 1
	s.statuses = s.statuses[i:]
	s.timestamps = s.timestamps[i:]
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
	// beacuse it is the current status
	if i >= len(s.timestamps) {
		i = len(s.timestamps) - 1
	}

	s.statuses = s.statuses[i:]
	s.timestamps = s.timestamps[i:]
}
