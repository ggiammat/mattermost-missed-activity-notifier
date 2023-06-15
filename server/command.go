package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/backend"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/model"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/output"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/userstatus"
	mm_model "github.com/mattermost/mattermost-server/v6/model"

	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
)

const CommandTrigger = "missedactivity"

func (p *MANPlugin) registerMANCommand() error {
	if err := p.API.RegisterCommand(&mm_model.Command{
		Trigger:          CommandTrigger,
		AutoComplete:     true,
		AutoCompleteHint: "[help|prefs|stats]",
		AutoCompleteDesc: "Configure the Missed Activity Plugin",
	}); err != nil {
		return errors.Wrap(err, "failed to register the command")
	}

	return nil
}

func commandStats(user *model.User, args []string, backend *backend.MattermostBackend, manRunStats *MANRunStats, userstatus *userstatus.UserStatusTracker) (string, error) {
	if !user.IsAdmin() {
		return "Only administrators can see stats", nil
	}
	out := output.PrintUserStatuses(userstatus, backend, manRunStats.sentEmailStats)

	buf := new(bytes.Buffer)
	for _, rl := range manRunStats.runLogs {
		fmt.Fprintf(buf, "# Run #%d (at: %s) (from: %s) (to: %s)\n",
			rl.numRun,
			rl.executionTime.Format("Jan 02 15:04"),
			rl.from.Format("Jan 02 15:04"),
			rl.to.Format("Jan 02 15:04"))
		for _, tl := range rl.textLogs {
			fmt.Fprintf(buf, "```%s```\n", tl)
		}
	}

	return fmt.Sprintf("# Users Report\n```%s```\n# Run Logs\n%s", out, buf.String()), nil

}

func commandPrefs(user *model.User, args []string, backend *backend.MattermostBackend) (string, error) {

	if len(args) == 1 {
		switch args[0] {
		case "show":
			out := fmt.Sprintf("Plugin Enabled: %t\nNotify replies in not followed posts (notify-replies-not-followed): %t\nShow count of replies in not followed posts (count-replies-not-followed): %t\nShow count of messages notified by Mattermost (count-notified-by-mm): %t\nShow count of previously notified messages by this plugin (count-previous-notified): %t\n",
				user.MANPreferences.Enabled,
				user.MANPreferences.NotifyRepliesInNotFollowedThreads,
				user.MANPreferences.IncludeCountOfRepliesInNotFollowedThreads,
				user.MANPreferences.InlcudeCountOfMessagesNotifiedByMM,
				user.MANPreferences.IncludeCountPreviouslyNotified)
			return out, nil
		case "reset":
			backend.ResetPreferenceEnabled(user)
			return "plugin reset", nil
		}
	}

	if len(args) == 2 {
		switch args[0] {
		case "enabled":
			if args[1] == "true" {
				backend.SetPreferenceEnabled(user, true)
				return "plugin enabled", nil
			}
			if args[1] == "false" {
				backend.SetPreferenceEnabled(user, false)
				return "plugin disabled", nil
			}
		case "notify-replies-not-followed":
			if args[1] == "true" {
				backend.SetPreferenceNotifyRepliesNotFollowed(user, true)
				return "notify-replies-not-followed enabled", nil
			}
			if args[1] == "false" {
				backend.SetPreferenceNotifyRepliesNotFollowed(user, false)
				return "notify-replies-not-followed disabled", nil
			}
		case "count-replies-not-followed":
			if args[1] == "true" {
				backend.SetPreferenceCountRepliesNotFollowed(user, true)
				return "count-replies-not-followed enabled", nil
			}
			if args[1] == "false" {
				backend.SetPreferenceCountRepliesNotFollowed(user, false)
				return "count-replies-not-followed disabled", nil
			}
		case "count-notified-by-mm":
			if args[1] == "true" {
				backend.SetPreferenceCountNotifiedByMM(user, true)
				return "count-notified-by-mm enabled", nil
			}
			if args[1] == "false" {
				backend.SetPreferenceCountNotifiedByMM(user, false)
				return "count-notified-by-mm disabled", nil
			}
		case "count-previous-notified":
			if args[1] == "true" {
				backend.SetPrefCountPreviouslyNotified(user, true)
				return "count-previous-notified enabled", nil
			}
			if args[1] == "false" {
				backend.SetPrefCountPreviouslyNotified(user, false)
				return "count-previous-notified disabled", nil
			}
		}
	}

	return "invalid arguments", nil
}

func (p *MANPlugin) executeCommandImpl(userID string, command string, args []string) (string, error) {

	user, uErr := p.backend.GetUser(userID)

	if uErr != nil {
		return "", errors.Wrap(uErr, "error getting user")
	}

	switch command {
	case "prefs":
		return commandPrefs(user, args, p.backend)
	case "help":
		readme := p.backend.GetReadmeContent()
		if strings.Index(readme, "## Admin Configuration") > 0 {
			readme = readme[:strings.Index(readme, "## Admin Configuration")]
		}
		helpMsg := fmt.Sprintf("%s\n\n---\n### Look at https://github.com/ggiammat/mattermost-missed-activity-notifier for additional documentation", readme)
		return helpMsg, nil
	case "stats":
		return commandStats(user, args, p.backend, p.manRunStats, p.userStatuses)
	}
	return "Specify a command: 'status', 'enable', 'disable'", nil

}

// Mattermost Hook
func (p *MANPlugin) ExecuteCommand(c *plugin.Context, args *mm_model.CommandArgs) (*mm_model.CommandResponse, *mm_model.AppError) {

	user, uErr := p.backend.GetUser(args.UserId)

	if uErr != nil {
		return &mm_model.CommandResponse{}, mm_model.NewAppError("MANAppError", "command error", nil, "error getting user", 1).Wrap(uErr)
	}

	tokens := strings.Split(strings.Trim(args.Command, " "), " ")

	if len(tokens) < 2 {
		return &mm_model.CommandResponse{Text: "Command not specified"}, nil
	}

	res, err := p.executeCommandImpl(user.Id, tokens[1], tokens[2:])

	if err != nil {
		return &mm_model.CommandResponse{Text: res}, mm_model.NewAppError("MANAppError", "command error", nil, "error executing command", 1).Wrap(err)
	} else {
		return &mm_model.CommandResponse{Text: res}, nil
	}
}
