package userstatus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func getTestingStatusHistory() *UserStatusHistory {
	return &UserStatusHistory{
		statuses: []UserStatus{Online, Offline, Online, Away, Online},
		timestamps: []int64{
			time.Date(2021, time.Month(2), 21, 8, 30, 0, 0, time.UTC).UnixMilli(),
			time.Date(2021, time.Month(2), 21, 10, 10, 0, 0, time.UTC).UnixMilli(),
			time.Date(2021, time.Month(2), 21, 10, 50, 0, 0, time.UTC).UnixMilli(),
			time.Date(2021, time.Month(2), 21, 12, 20, 0, 0, time.UTC).UnixMilli(),
			time.Date(2021, time.Month(2), 21, 13, 10, 0, 0, time.UTC).UnixMilli(),
		},
	}
}

func getTestingStatusHistory2() *UserStatusHistory {
	return &UserStatusHistory{
		statuses: []UserStatus{Offline, Online, Away, Online},
		timestamps: []int64{
			time.Date(2021, time.Month(2), 21, 6, 30, 0, 0, time.UTC).UnixMilli(),
			time.Date(2021, time.Month(2), 21, 8, 0, 0, 0, time.UTC).UnixMilli(),
			time.Date(2021, time.Month(2), 21, 13, 0, 0, 0, time.UTC).UnixMilli(),
			time.Date(2021, time.Month(2), 21, 14, 0, 0, 0, time.UTC).UnixMilli(),
		},
	}
}

func getTestingStatusHistory3() *UserStatusHistory {
	return &UserStatusHistory{
		statuses:   []UserStatus{Online, Away, Online},
		timestamps: []int64{1685616261149, 1685616559534, 1685616636713},
	}
}

func getTestingAllUserStatuses() UserStatusTracker {
	return UserStatusTracker{
		usersMap: map[string]*UserStatusHistory{"user1": getTestingStatusHistory(), "user2": getTestingStatusHistory2(), "user3": getTestingStatusHistory3()},
	}
}

func TestSameTimestamp(t *testing.T) {
	as := getTestingAllUserStatuses()
	assert.Equal(t, Away, as.GetStatusForUserAtTime("user3", time.UnixMilli(1685616559534)))
}

func TestAllUserStatusesGet(t *testing.T) {
	as := getTestingAllUserStatuses()
	assert.Equal(t, Online, as.GetStatusForUserAtTime("user1", time.Date(2021, time.Month(2), 21, 11, 20, 0, 0, time.UTC)))
	assert.Equal(t, Away, as.GetStatusForUserAtTime("user2", time.Date(2021, time.Month(2), 21, 13, 20, 0, 0, time.UTC)))
	assert.Equal(t, Unknown, as.GetStatusForUserAtTime("user2", time.Date(2021, time.Month(2), 21, 5, 00, 0, 0, time.UTC)))
	assert.Equal(t, Online, as.GetStatusForUserAtTime("user1", time.Date(2021, time.Month(2), 21, 15, 00, 0, 0, time.UTC)))
}

func TestAllUserStatusesSet(t *testing.T) {
	as := getTestingAllUserStatuses()

	// set a new status
	err := as.setStatusAtTime("user2", "offline", time.Date(2021, time.Month(2), 21, 14, 40, 0, 0, time.UTC).UnixMilli())
	assert.Nil(t, err)
	assert.Equal(t, 5, len(as.usersMap["user2"].statuses))
	assert.Equal(t, Offline, as.usersMap["user2"].statuses[4])

	// try to set a status in the past
	err = as.setStatusAtTime("user1", "offline", time.Date(2021, time.Month(2), 21, 12, 20, 0, 0, time.UTC).UnixMilli())
	assert.EqualError(t, err, "cannot set status for a time older than the last timestamp")

	// add a new user
	as.setStatusAtTime("user3", "online", time.Date(2021, time.Month(2), 21, 13, 20, 0, 0, time.UTC).UnixMilli())
	assert.Equal(t, 3, len(as.usersMap))

}

func TestSetStatus(t *testing.T) {

	sh := getTestingStatusHistory()

	err := sh.SetStatusAt(Online, time.Date(2021, time.Month(2), 21, 12, 20, 0, 0, time.UTC).UnixMilli())

	assert.EqualError(t, err, "cannot set status for a time older than the last timestamp")

	err = sh.SetStatusAt(Online, time.Date(2021, time.Month(2), 21, 14, 00, 0, 0, time.UTC).UnixMilli())

	assert.Nil(t, err)
	assert.Equal(t, 5, len(sh.statuses))
	assert.Equal(t, 5, len(sh.timestamps))

	err = sh.SetStatusAt(Away, time.Date(2021, time.Month(2), 21, 14, 00, 0, 0, time.UTC).UnixMilli())

	assert.Nil(t, err)
	assert.Equal(t, 6, len(sh.statuses))
	assert.Equal(t, 6, len(sh.timestamps))
	assert.Equal(t, Away, sh.statuses[5])
}

func TestUserStatusHistory_getStatusAt(t *testing.T) {

	sh := getTestingStatusHistory()

	assert.Equal(t,
		Unknown,
		sh.getStatusAt(time.Date(2021, time.Month(2), 21, 6, 20, 0, 0, time.UTC)), "")
	assert.Equal(t,
		Offline,
		sh.getStatusAt(time.Date(2021, time.Month(2), 21, 10, 30, 0, 0, time.UTC)), "")
	assert.Equal(t,
		Online,
		sh.getStatusAt(time.Date(2021, time.Month(2), 21, 14, 30, 0, 0, time.UTC)), "")
}

func TestUserStatusHistory_clearOlderThan(t *testing.T) {

	sh := getTestingStatusHistory()
	sh.clearHistoyOlderThan(time.Date(2021, time.Month(2), 21, 7, 30, 0, 0, time.UTC))

	assert.Equal(t, 5, len(sh.timestamps))
	assert.Equal(t, 5, len(sh.statuses))
	assert.Equal(t, time.Date(2021, time.Month(2), 21, 8, 30, 0, 0, time.UTC).UnixMilli(), sh.timestamps[0])

	sh = getTestingStatusHistory()
	sh.clearHistoyOlderThan(time.Date(2021, time.Month(2), 21, 11, 00, 0, 0, time.UTC))

	assert.Equal(t, 2, len(sh.timestamps))
	assert.Equal(t, 2, len(sh.statuses))
	assert.Equal(t, time.Date(2021, time.Month(2), 21, 12, 20, 0, 0, time.UTC).UnixMilli(), sh.timestamps[0])

	sh = getTestingStatusHistory()
	sh.clearHistoyOlderThan(time.Date(2021, time.Month(2), 21, 13, 00, 0, 0, time.UTC))

	assert.Equal(t, 1, len(sh.timestamps))
	assert.Equal(t, 1, len(sh.statuses))
	assert.Equal(t, time.Date(2021, time.Month(2), 21, 13, 10, 0, 0, time.UTC).UnixMilli(), sh.timestamps[0])

	sh = getTestingStatusHistory()
	sh.clearHistoyOlderThan(time.Date(2021, time.Month(2), 21, 14, 00, 0, 0, time.UTC))

	assert.Equal(t, 1, len(sh.timestamps))
	assert.Equal(t, 1, len(sh.statuses))
	assert.Equal(t, time.Date(2021, time.Month(2), 21, 13, 10, 0, 0, time.UTC).UnixMilli(), sh.timestamps[0])
}
