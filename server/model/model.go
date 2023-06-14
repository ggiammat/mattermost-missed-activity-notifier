package model

import (
	"fmt"
	"strings"
	"time"
)

type MANUserPreferences struct {
	Enabled                                   bool
	NotifyRepliesInNotFollowedThreads         bool
	IncludeCountOfRepliesInNotFollowedThreads bool
	InlcudeCountOfMessagesNotifiedByMM        bool
	IncludeCountPreviouslyNotified            bool
}

type TeamMissedActivity struct {
	User           *User
	Team           *Team
	UnreadChannels []ChannelMissedActivity
	Logs           []string
}

func (uma *TeamMissedActivity) AppendLog(message string, a ...any) {
	uma.Logs = append(uma.Logs, fmt.Sprintf(message, a...))
}

// posts coming from the db
type Post struct {
	Id        string
	Message   string
	AuthorId  string
	CreatedAt time.Time
	RootId    string
	Type      string
}

func (p *Post) IsRoot() bool {
	return p.RootId == ""
}

/*
type UnreadReply struct {
	Message string
	Auhtor  *User
	Type    string
	Time    time.Time
}
*/

type UnreadConversation struct {
	//Message             string
	//Auhtor              *User
	//Type                string
	//Time                time.Time
	Following bool
	//HasMentions         bool
	IsRootMessageUnread bool
	//ContainsMentions    bool
	RootPost          *Post
	Replies           []*Post
	MostRecentMessage time.Time
}

func (up *UnreadConversation) IsAuthor(user *User) bool {
	return user.Id == up.RootPost.AuthorId
}

func NewUnreadConversation(rootPost *Post, following bool, rootPostUnread bool) *UnreadConversation {
	return &UnreadConversation{
		RootPost:            rootPost,
		Following:           following,
		IsRootMessageUnread: rootPostUnread,
		Replies:             []*Post{},
		MostRecentMessage:   rootPost.CreatedAt,
	}
}

func (uc *UnreadConversation) AppendReply(post *Post) {
	uc.Replies = append(uc.Replies, post)
	if uc.MostRecentMessage.Before(post.CreatedAt) {
		uc.MostRecentMessage = post.CreatedAt
	}
}

type User struct {
	Id             string
	Username       string
	FirstName      string
	LastName       string
	Email          string
	EmailVerified  bool
	Active         bool
	Roles          []string
	Status         string
	IsBot          bool
	EmailsEnabled  bool
	Image          []byte
	MANPreferences MANUserPreferences
}

func (u *User) IsAdmin() bool {
	for _, a := range u.Roles {
		if a == "system_admin" {
			return true
		}
	}
	return false
}

func (u *User) DisplayName() string {

	if u.FirstName != "" || u.LastName != "" {
		return fmt.Sprintf("%s %s", u.FirstName, u.LastName)
	}

	return u.Username
}

type Team struct {
	Id   string
	Name string
}

var (
	// fake team to handle direct messages. Direct Messages does not belong
	// to a particular Team, so to manage them uniformely we use this special team
	DIRECT_MESSAGES_FAKE_TEAM = &Team{Name: "Direct Messages", Id: ""}
)

type Channel struct {
	Id          string
	DisplayName string
	Members     []string
	Type        string
	TeamId      string
}

func (c *Channel) IsDirect() bool {
	return c.Type == "D"
}

type ChannelMissedActivity struct {
	Channel                    *Channel
	User                       *User
	UnreadConversations        []*UnreadConversation
	Logs                       []string
	RepliesInNotFollowingConvs int
	NotifiedByMMMessages       int
	PreviouslyNotified         int
}

func NewChannelMissedActivity(channel *Channel, user *User) *ChannelMissedActivity {
	return &ChannelMissedActivity{
		Channel:                    channel,
		User:                       user,
		UnreadConversations:        []*UnreadConversation{},
		RepliesInNotFollowingConvs: 0,
		NotifiedByMMMessages:       0,
		PreviouslyNotified:         0,
	}
}

func (cma *ChannelMissedActivity) AppendLog(message string, a ...any) {
	cma.Logs = append(cma.Logs, fmt.Sprintf(message, a...))
}

func (cma *ChannelMissedActivity) GetChannelName() string {
	return cma.Channel.GetChannelName(cma.User)
}

type ChannelMembership struct {
	Channel      *Channel
	User         *User
	LastReadPost time.Time
	NotifyProps  map[string]string
}

func (cm *ChannelMembership) IsMuted() bool {
	return cm.NotifyProps["mark_unread"] == "mention"
}

func (ch *Channel) GetChannelName(user *User) string {
	if ch.Type == "D" || ch.Type == "G" {
		users := ch.Members

		usernames := []string{}
		for k := 0; k < len(users); k++ {
			if users[k] == user.DisplayName() {
				continue
			}
			usernames = append(usernames, users[k])
		}
		return fmt.Sprintf("Chat with %s", strings.Join(usernames, ", "))
	}

	if ch.DisplayName != "" {
		return ch.DisplayName
	}

	return "INVALID NAME"

}