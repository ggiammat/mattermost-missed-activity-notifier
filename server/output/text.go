package output

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/backend"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/model"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/userstatus"
	"github.com/mergestat/timediff"
	"github.com/olekukonko/tablewriter"
)

func PrintUserStatuses(userStatus *userstatus.UserStatusTracker, backend *backend.MattermostBackend, sentEmailsStats map[string][]time.Time) string {
	w := new(bytes.Buffer)

	ids := userStatus.GetTrackerUserIds()

	table := tablewriter.NewWriter(w)
	table.SetAutoWrapText(false)
	table.SetHeader([]string{"P", "V", "E", "A", "User", "Statuses", "Tot Emails", "Last Email"})

	for _, id := range ids {
		user, errU := backend.GetUser(id)
		if errU != nil {
			fmt.Fprintf(w, "error getting user with id %s", id)
		}

		if user.IsBot {
			continue
		}

		p := ""
		if !user.MANPreferences.Enabled {
			p = "x"
		}
		v := ""
		if !user.EmailVerified {
			v = "x"
		}
		e := ""
		if !user.EmailsEnabled {
			e = "x"
		}
		a := ""
		if !user.Active {
			a = "x"
		}

		emailTot := ""
		emailLast := ""

		if entry, ok := sentEmailsStats[user.Id]; ok {
			emailTot = strconv.Itoa(len(entry))
			emailLast = entry[len(entry)-1].Format(time.RFC822)
		}

		timestamps, statuses := userStatus.GetUserStatusHistory(id)
		statusStr := make([]string, len(timestamps))

		for i, t := range timestamps {
			statusStr[i] = fmt.Sprintf("[%d] %s", statuses[i], time.UnixMilli(t).Format("Mon 2 15:04"))
		}

		row := []string{p, v, e, a, user.DisplayName(), strings.Join(statusStr, " > "), emailTot, emailLast}
		table.Append(row)
	}
	table.Render()
	fmt.Fprintf(w, "P: Missed Activity Enabled / V: Email Verified / E: Email Notifications Enabled / A: User is Active\n(Bot users are excluded)\n")

	return w.String()
}

func PrintTeamMissedActivity(backend *backend.MattermostBackend, missedActivity *model.TeamMissedActivity) string {
	w := new(bytes.Buffer)

	fmt.Fprintf(w, "\n\n▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀\n")
	fmt.Fprintf(w, "▀ %s in %s\n", missedActivity.User.Username, missedActivity.Team.Name)
	fmt.Fprintf(w, "▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀\n\n")

	for _, crs := range missedActivity.UnreadChannels {
		fmt.Fprintf(w, "In %s (%s)\n", crs.GetChannelName(), crs.Channel.Type)
		fmt.Fprintf(w, "▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔\n\n")

		for j := 0; j < len(crs.UnreadConversations); j++ {
			up := crs.UnreadConversations[j]
			author, _ := backend.GetUser(up.RootPost.AuthorId)

			str5 := timediff.TimeDiff(up.RootPost.CreatedAt)
			followingIcon := ""
			if up.Following {
				followingIcon = "🔀 "
			}
			mentionIcon := ""
			if model.MessageContainsMentions(up.RootPost.Message, author.Username) {
				mentionIcon = "🙊 "
			}
			typeIcon := ""
			if up.RootPost.Type != "" {
				typeIcon = "🎮 "
			}
			rootUnreadIcon := ""
			if up.IsRootMessageUnread {
				rootUnreadIcon = "🌟 "
			}

			conversationText := up.RootPost.Message
			fmt.Fprintf(w, "┊ %s %s wrote:  %s%s%s%s\n", author.Username, str5, followingIcon, mentionIcon, typeIcon, rootUnreadIcon)
			fmt.Fprintf(w, "┊  | %s [at: %d]\n", up.RootPost.Message, up.RootPost.CreatedAt.UnixMilli())
			if len(up.Replies) > 0 {
				for _, r := range up.Replies {
					fmt.Fprintf(w, "┊  |   > %s [at: %d]\n", r.Message, r.CreatedAt.UnixMilli())
					author, _ := backend.GetUser(r.AuthorId)
					conversationText = fmt.Sprintf("%s<br/>  > <strong>%s</strong> replied: %s", conversationText, author.Username, r.Message)
				}
			}

			fmt.Fprintf(w, "\n")
		}

		if crs.PreviouslyNotified > 0 {
			fmt.Fprintf(w, "+%d messages previously notified\n", crs.PreviouslyNotified)
		}

		if crs.RepliesInNotFollowingConvs > 0 {
			fmt.Fprintf(w, "+%d messages in not followed threads\n", crs.RepliesInNotFollowingConvs)
		}

		if crs.NotifiedByMMMessages > 0 {
			fmt.Fprintf(w, "+%d messages already notified by email by Mattermost\n", crs.NotifiedByMMMessages)
		}

		fmt.Fprintf(w, "\n")

		if len(crs.Logs) > 0 {
			for _, m := range crs.Logs {
				fmt.Fprintf(w, "░ %s\n", m)
			}
		}

		fmt.Fprintf(w, "\n")
	}

	return w.String()
}
