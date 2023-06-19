package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	mm_model "github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/oleiade/reflections"
	"github.com/pkg/errors"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/backend"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/model"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/output"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/userstatus"
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

func commandStats(user *model.User, _ []string, backend *backend.MattermostBackend, manRunStats *MANRunStats, userstatus *userstatus.UserStatusTracker) (string, error) {
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

	return fmt.Sprintf("# Run Logs\n%s\n# Users Report\n```%s```", buf.String(), out), nil
}

func commandResetAll(user *model.User, backend *backend.MattermostBackend) (string, error) {
	if !user.IsAdmin() {
		return "Only administrators can reset all user preferences", nil
	}

	backend.ResetAllUserPrefernces()

	return "All user preferences reset", nil
}

func commandPrefs(user *model.User, args []string, backend *backend.MattermostBackend) (string, error) {
	if len(args) == 1 {
		switch args[0] {
		case "show":
			out := "### Current preferences:\n"
			res, _ := reflections.Items(user.MANPreferences)
			for k, v := range res {
				out += fmt.Sprintf("  - **%s**: %v\n", k, v)
			}
			return out, nil

		case "reset":
			err := backend.ResetPreferences(user)
			if err != nil {
				return "", err
			}
			return "preferences reset", nil
		}
	}

	if len(args) == 2 {
		field := args[0]
		has, _ := reflections.HasField(user.MANPreferences, field)

		if !has {
			return fmt.Sprintf("invalid preference name '%s'", field), nil
		}

		currValue, _ := reflections.GetField(user.MANPreferences, field)
		var newVal any

		// convert new value from string to the field type (at the moment)
		// all preferences are bool, but in future new preferences could
		// be added
		switch currValue.(type) {
		case string:
			newVal = args[1]
		case bool:
			boolValue, errB := strconv.ParseBool(args[1])
			if errB != nil {
				return "", errB
			}
			newVal = boolValue
		}

		errS := backend.SetUserPreference(user, field, newVal)
		if errS != nil {
			return "", errS
		}

		return fmt.Sprintf("preference %s = %t", field, newVal), nil

	}

	return "invalid number of arguments", nil
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
			//nolint:gocritic
			readme = readme[:strings.Index(readme, "## Admin Configuration")]
		}
		helpMsg := fmt.Sprintf("%s\n\n---\n### Look at https://github.com/ggiammat/mattermost-missed-activity-notifier for additional documentation", readme)
		return helpMsg, nil
	case "stats":
		return commandStats(user, args, p.backend, p.manRunStats, p.userStatuses)
	case "reset-all-user-prefs":
		return commandResetAll(user, p.backend)
	}
	return "Invalid command", nil
}

// Mattermost Hook
func (p *MANPlugin) ExecuteCommand(_ *plugin.Context, args *mm_model.CommandArgs) (*mm_model.CommandResponse, *mm_model.AppError) {
	user, uErr := p.backend.GetUser(args.UserId)

	if uErr != nil {
		return &mm_model.CommandResponse{}, mm_model.NewAppError("MANAppError", "command error", nil, "error getting user", 1).Wrap(uErr)
	}

	tokens := strings.Split(strings.Trim(args.Command, " "), " ")

	if len(tokens) < 2 {
		return &mm_model.CommandResponse{Text: "Command not specified"}, nil
	}

	res, err := p.executeCommandImpl(user.ID, tokens[1], tokens[2:])

	if err != nil {
		return &mm_model.CommandResponse{Text: res}, mm_model.NewAppError("MANAppError", "command error", nil, "error executing command", 1).Wrap(err)
	}

	return &mm_model.CommandResponse{Text: res}, nil
}
