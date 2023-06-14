package main

import (
	"fmt"
	"time"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/man"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/output"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/userstatus"
	"github.com/mattermost/mattermost-plugin-api/cluster"
	"github.com/pkg/errors"
)

func (p *MANPlugin) deactivateMANJob() error {
	if p.manJob != nil {
		if err := p.manJob.Close(); err != nil {
			return errors.Wrap(err, "Error deactivating job")
		}
		p.backend.LogDebug("MAN Job deactivated")

	}
	return nil
}

func (p *MANPlugin) activateMANJob() error {

	p.backend.LogDebug("Activating MAN Job")

	lastNotifiedTime, errA := p.backend.GetLastNotifiedTimestamp()

	if errA != nil {
		return errors.Wrap(errA, "Error getting last notified timestamp")
	}

	// if last run + run interaval < now execute immediately
	if lastNotifiedTime.Add(time.Minute * time.Duration(p.getConfiguration().RunInterval)).Before(time.Now()) {
		p.backend.LogDebug("Run MAN immediately because last notified + run interval < now")
		p.MANJob()
	}

	job, cronErr := cluster.Schedule(
		p.API,
		"BackgroundJob",
		cluster.MakeWaitForRoundedInterval(time.Duration(p.getConfiguration().RunInterval)*time.Minute),
		p.MANJob,
	)

	if cronErr != nil {
		return errors.Wrap(cronErr, "failed to schedule background job")
	}
	p.manJob = job

	return nil
}

func (p *MANPlugin) CleanMANStats() {

	length := len(p.manRunStats.runLogs)
	if length > p.configuration.RunStatsToKeep {
		p.manRunStats.runLogs = p.manRunStats.runLogs[length-p.configuration.RunStatsToKeep:]
	}

	// we use the same limit also for emails
	for k, v := range p.manRunStats.sentEmailStats {
		length := len(p.manRunStats.sentEmailStats[k])
		if length > p.configuration.RunStatsToKeep {
			p.manRunStats.sentEmailStats[k] = v[length-p.configuration.RunStatsToKeep:]
		}
	}
}

func (p *MANPlugin) MANJob() {

	// 1. calculate the time range in which run
	lower, errT := p.backend.GetLastNotifiedTimestamp()
	if errT != nil {
		p.backend.LogError("error running MANJob while retrieving the last notfied timestamp: %s", errT)
		return
	}

	upper := time.Now().Add(time.Minute * (-time.Duration(p.configuration.IgnoreMessagesNewerThan)))

	// 2. run MAN. This will return a list of TeamMissedActivity objects
	res, err := man.RunMAN(p.backend, p.userStatuses, &man.MissedActivityOptions{
		LastNotifiedTimestamp: lower,
		To:                    upper,
	})

	if err != nil {
		p.backend.LogError("Error running MAN: %s", err)
		return
	}

	execLogs := &MANRunLog{
		numRun:        p.manRunStats.numRuns + 1,
		executionTime: time.Now(),
		from:          lower,
		to:            upper,
		textLogs:      []string{},
		htmlEmails:    []string{},
	}

	// 3. for each TeamMissedActivity
	//    - render in plain text and save in stats
	//    - build the email html text and save in stats
	//    - send the email

	for _, r := range res {

		// record text logs
		out := output.PrintTeamMissedActivity(p.backend, r)
		execLogs.textLogs = append(execLogs.textLogs, out)

		emailConfig := &output.EmailTemplateProps{
			SubTitle:    p.configuration.EmailSubTitle,
			ButtonText:  p.configuration.EmailButtonText,
			FooterLine1: p.configuration.EmailFooterLine1,
			FooterLine2: p.configuration.EmailFooterLine2,
			FooterLine3: p.configuration.EmailFooterLine3,
		}

		subject, email, errM := output.BuildHTMLEmail(p.backend, r, emailConfig)
		if errM != nil {
			p.backend.LogError("Cannot send email! Error building email: %s", errM)
			continue
		}

		if email != "" {

			// record email logs
			execLogs.htmlEmails = append(execLogs.htmlEmails, fmt.Sprintf("To: %s Subject: %s Body: <br>%s", r.User.Email, subject, email))
			if entry, ok := p.manRunStats.sentEmailStats[r.User.Id]; ok {
				p.manRunStats.sentEmailStats[r.User.Id] = append(entry, time.Now())
			} else {
				p.manRunStats.sentEmailStats[r.User.Id] = []time.Time{time.Now()}
			}

			// send email
			if !p.configuration.DryRun {
				errE := p.backend.SendEmailToUser(r.User, subject, email)
				if errE != nil {
					p.backend.LogError("Cannot send email! Error sending email: %s", errE)
				}
			}
		}
	}

	// notice in the logs if running in dry run mode
	if p.configuration.DryRun {
		p.backend.LogWarn("MAN plugin did not sent emails because it is running in DryRun mode. Please disable it to start sending emails")
	}

	// 4. update the stats with the new run
	p.manRunStats.runLogs = append(p.manRunStats.runLogs, *execLogs)
	p.manRunStats.numRuns++

	// 5. record the last notified timestamp in the db
	p.backend.SetLastNotifiedTimestamp(upper)

	// 6. housekeeping
	p.CleanMANStats()
	// remove statuses older than the last run because we will not need them
	userstatus.ClearStatusesOlderThan(p.userStatuses, time.Now().Add(time.Hour*(-time.Duration(p.configuration.KeepStatusHistoryInterval))))

}
