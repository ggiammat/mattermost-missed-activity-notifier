package model

import (
	"strings"
	"time"
)

func MessageContainsMentions(message string, username string) bool {
	return strings.Contains(message, "@all") || strings.Contains(message, "@channel") || strings.Contains(message, "@"+username)
}

func GetMostRecent(a time.Time, b time.Time) time.Time {
	if a.Before(b) {
		return b
	} else {
		return a
	}
}
