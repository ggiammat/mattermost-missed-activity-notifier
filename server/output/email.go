package output

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	"github.com/ggiammat/mattermost-missed-activity-notifier/server/backend"
	"github.com/ggiammat/mattermost-missed-activity-notifier/server/model"
)

type EmailTemplateProps struct {
	SubTitle    string
	ButtonText  string
	FooterLine1 string
	FooterLine2 string
	FooterLine3 string
}

type postData struct {
	SenderName               string
	ChannelName              string
	Message                  template.HTML
	SenderPhoto              template.URL
	PostPhoto                string
	Time                     string
	ShowChannelIcon          bool
	OtherChannelMembersCount int
	MessageAttachments       []*string
	Link                     template.URL
	AlreadyRead              bool
}

type conversationData struct {
	RootPost   postData
	Replies    []postData
	NumReplies int
}

type channelData struct {
	ChannelName                    string
	ShowChannelIcon                bool
	OtherChannelMembersCount       int
	Conversations                  []*conversationData
	NumRepliesInNotFollowedThreads int
	NumNotifiedByMM                int
	NumPreviouslyNotified          int
}

func buildChannelData(cma *model.ChannelMissedActivity, conversationsData []*conversationData) *channelData {
	return &channelData{
		ChannelName:                    cma.GetChannelName(),
		ShowChannelIcon:                true,
		NumRepliesInNotFollowedThreads: cma.RepliesInNotFollowingConvs,
		NumNotifiedByMM:                cma.NotifiedByMMMessages,
		NumPreviouslyNotified:          cma.PreviouslyNotified,
		Conversations:                  conversationsData,
	}
}

type templateData struct {
	Props map[string]any
	HTML  map[string]string
}

func formatMessage(message string, siteUrl string) template.HTML {

	postMessage := html.EscapeString(message)
	mdPostMessage, mdErr := MarkdownToHTML(postMessage, siteUrl)
	if mdErr != nil {
		mdPostMessage = postMessage
	}

	return template.HTML(mdPostMessage)
}

func toBase64(bytes []byte) template.URL {
	var base64Encoding string

	// Determine the content type of the image file
	mimeType := http.DetectContentType(bytes)

	// Prepend the appropriate URI scheme header depending
	// on the MIME type. The full scheme would be "data:image/jpeg;base64,"
	// but we remove the first part "data:image/" (and add it in the template
	// directly) because it is considered unsafe by html/template and would
	// not be rendered in the template.
	switch mimeType {
	case "image/jpeg":
		base64Encoding += "data:image/jpeg;base64,"
	case "image/png":
		base64Encoding += "data:image/png;base64,"
	}

	// Append the base64 encoded output
	base64Encoding += base64.StdEncoding.EncodeToString(bytes)
	//nolint:gosec
	return template.URL(base64Encoding)
}

// adaptation of the MarkdownToHTML() function in Mattermost's  server/channels/utils/markdown.go
// to mimic the emails sent by Mattermost
var relLinkReg = regexp.MustCompile(`\[(.*)]\((/.*)\)`)
var blockquoteReg = regexp.MustCompile(`^|\n(&gt;)`)

func MarkdownToHTML(markdown, siteURL string) (string, error) {
	// Turn relative links into absolute links
	absLinkMarkdown := relLinkReg.ReplaceAllStringFunc(markdown, func(s string) string {
		return relLinkReg.ReplaceAllString(s, "[$1]("+siteURL+"$2)")
	})

	// Unescape any blockquote text to be parsed by the markdown parser.
	markdownClean := blockquoteReg.ReplaceAllStringFunc(absLinkMarkdown, func(s string) string {
		return html.UnescapeString(s)
	})

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
	)

	var b strings.Builder

	err := md.Convert([]byte(markdownClean), &b)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func BuildHTMLEmail(backend *backend.MattermostBackend, missedActivity *model.TeamMissedActivity, props *EmailTemplateProps) (string, string, error) {
	serverName := backend.GetServerName()
	serverURL := backend.GetServerURL()

	// closure function to build the link to messages
	buildMessageLink := func(post *model.Post) template.URL {
		teamName := missedActivity.Team.Name

		if missedActivity.Team.ID == "" {
			// although direct messages don't belong to any team, we have to specify a team name in the
			// url. We choose the first team the user belongs to
			teams, _ := backend.GetTeamsForUser(missedActivity.User.ID)
			if len(teams) < 1 {
				backend.LogError("Cannot build a link for direct message %s: user is not member of any team", post.ID)
				//nolint:gosec
				return template.URL(serverURL)
			}
			teamName = teams[0].Name
		}
		//nolint:gosec
		return template.URL(fmt.Sprintf("%s/%s/pl/%s", serverURL, strings.ToLower(teamName), post.ID))
	}

	t, err := template.ParseFiles(filepath.Join(backend.GetTemplatesPath(), "email-content.html"))
	if err != nil {
		return "", "", errors.Wrap(err, "Error loading template")
	}

	title := fmt.Sprintf("Missed Activity in the %s team", missedActivity.Team.Name)
	if missedActivity.Team == model.DirectMessagesFakeTeam {
		title = fmt.Sprintf("Missed Direct Messages in %s", serverName)
	}

	data := templateData{
		Props: map[string]any{
			"SiteURL":          serverURL,
			"EmailTitle":       title,
			"ButtonURL":        serverURL,
			"EmailSubTitle":    props.SubTitle,
			"EmailButton":      props.ButtonText,
			"EmailFooterLine1": props.FooterLine1,
			"EmailFooterLine2": props.FooterLine2,
			"EmailFooterLine3": props.FooterLine3,
		},
		HTML: map[string]string{},
	}

	nConversations := 0
	channels := []*channelData{}

	for _, cma := range missedActivity.UnreadChannels {
		conversationsData := []*conversationData{}

		for _, conv := range cma.UnreadConversations {
			author, _ := backend.GetUser(conv.RootPost.AuthorID)

			p := postData{
				SenderName:  author.DisplayName(),
				Message:     formatMessage(conv.RootPost.Message, serverURL),
				Time:        conv.RootPost.CreatedAt.Format(time.RFC822),
				SenderPhoto: toBase64(author.Image),
				Link:        buildMessageLink(conv.RootPost),
				AlreadyRead: !conv.IsRootMessageUnread,
			}

			cv := &conversationData{RootPost: p}

			replies := []postData{}

			for _, rep := range conv.Replies {
				author, _ := backend.GetUser(rep.AuthorID)

				p := postData{
					SenderName:  author.DisplayName(),
					Message:     formatMessage(rep.Message, serverURL),
					Time:        rep.CreatedAt.Format(time.RFC822),
					SenderPhoto: toBase64(author.Image),
					Link:        buildMessageLink(rep),
				}
				replies = append(replies, p)
			}

			cv.Replies = replies
			cv.NumReplies = len(cv.Replies)

			conversationsData = append(conversationsData, cv)
			nConversations++
		}

		// only add a channel if there is something to notify, otherwise an empty header will appear in the email
		if len(conversationsData) > 0 || cma.RepliesInNotFollowingConvs > 0 || cma.NotifiedByMMMessages > 0 || cma.PreviouslyNotified > 0 {
			//nolint:gosec
			channels = append(channels, buildChannelData(&cma, conversationsData))
		}
	}

	sort.Slice(channels, func(i, j int) bool {
		diff := len(channels[i].Conversations) - len(channels[j].Conversations)
		if diff != 0 {
			return diff > 0
		}
		return channels[i].ChannelName < channels[j].ChannelName
	})

	data.Props["Channels"] = channels

	// build email only if there is at least one conversation in one channel
	if nConversations > 0 {
		w := new(bytes.Buffer)
		errR := t.Execute(w, data)
		if errR != nil {
			return "", "", errors.Wrap(errR, "Error rendering html email template")
		}

		var subject string
		if missedActivity.Team.ID != "" {
			subject = fmt.Sprintf("[%s] Recent activity in %s", serverName, missedActivity.Team.Name)
		} else {
			subject = fmt.Sprintf("[%s] Unread direct messages", serverName)
		}
		return subject, w.String(), nil
	}

	return "", "", nil
}
