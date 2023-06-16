package model

import (
	"strings"
)

func MessageContainsMentions(message string, username string) bool {
	return strings.Contains(message, "@all") || strings.Contains(message, "@channel") || strings.Contains(message, "@"+username)
}
