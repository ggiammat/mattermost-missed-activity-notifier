package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/man"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/output"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

func (p *MANPlugin) printHistory(w http.ResponseWriter) {

	if len(p.manRunStats.runLogs) == 0 {
		return
	}

	for i := len(p.manRunStats.runLogs) - 1; i >= 0; i-- {
		rl := p.manRunStats.runLogs[i]
		fmt.Fprintf(w, "<h1>Run #%d (at: %s) (from: %s) (to: %s) <a href=\"http://localhost:8065/plugins/com.mattermost.missed-activity-notifier?run=true&from=%d&to=%d\">RERUN</a></h1>",
			rl.numRun,
			rl.executionTime.Format("Jan 02 15:04"),
			rl.from.Format("Jan 02 15:04"),
			rl.to.Format("Jan 02 15:04"),
			rl.from.UnixMilli(),
			rl.to.UnixMilli())

		for _, tl := range rl.textLogs {
			fmt.Fprintf(w, "<pre>%s</pre></br>", tl)
		}
		for _, tl := range rl.htmlEmails {
			fmt.Fprintf(w, "%s</br>", tl)
		}
	}
}

func (p *MANPlugin) manualRun(buf http.ResponseWriter, lower time.Time, upper time.Time) {
	lowerBound := time.UnixMilli(0)
	if p.configuration.NotifyOnlyNewMessagesFromStartup {
		lowerBound = p.startupTime
	}
	res, err := man.RunMAN(p.backend, p.userStatuses, &man.MissedActivityOptions{
		LowerBound:            lowerBound,
		LastNotifiedTimestamp: lower,
		UpperBound:            upper,
	})

	if err != nil {
		p.backend.LogError("Error running MAN: %s", err)
		return
	}

	for _, r := range res {
		out := output.PrintTeamMissedActivity(p.backend, r)
		fmt.Fprintf(buf, "<pre>%s</pre><br/>", out)
		emailConfig := &output.EmailTemplateProps{
			SubTitle:    p.configuration.EmailSubTitle,
			ButtonText:  p.configuration.EmailButtonText,
			FooterLine1: p.configuration.EmailFooterLine1,
			FooterLine2: p.configuration.EmailFooterLine2,
			FooterLine3: p.configuration.EmailFooterLine3,
		}

		subject, email, err := output.BuildHTMLEmail(p.backend, r, emailConfig)
		if err != nil {
			fmt.Fprintf(buf, "<strong>Error building email: %s</strong>", err)
		} else {
			fmt.Fprintf(buf, "<h2>to: %s sub: %s</h2>%s<br/>", r.User.Username, subject, email)

		}

	}
}

func (p *MANPlugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {

	//b, _ := p.API.GetBundlePath()

	/*
		f, err := ioutil.ReadFile(filepath.Join(b, "assets/templates/html-header.html"))
		if err != nil {
			fmt.Printf("Error reading html template:%s", err)
		}
	*/
	//fmt.Fprintf(w, string(f))
	fmt.Fprintf(w, "<a href=\"/plugins/com.mattermost.missed-activity-notifier?status\">STATUS</a> | <a href=\"/plugins/com.mattermost.missed-activity-notifier?history\">HISTORY</a> | Now %s |<br/>", time.Now().Format(time.RFC822))

	if r.URL.Query().Has("history") {
		p.printHistory(w)
	} else if r.URL.Query().Has("run") {
		from, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
		to, err := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
		if err != nil {
			fmt.Fprintf(w, "Error converting from and to")
			return
		}

		p.manualRun(w, time.UnixMilli(from), time.UnixMilli(to))

	} else if r.URL.Query().Has("status") {
		out := output.PrintUserStatuses(p.userStatuses, p.backend, p.manRunStats.sentEmailStats)
		fmt.Fprintf(w, "<pre>%s</pre><br/>", out)

		statuses, err := p.backend.GetUsersStatus()
		if err != nil {
			fmt.Printf("Error getting users for tracking their status: %s", err)
			return
		}

		for k, v := range statuses {
			u, _ := p.backend.GetUser(k)
			if u.IsBot {
				continue
			}
			fmt.Fprintf(w, "%s %s<br/>", u.Username, v)
		}
	}
}
