package handlers

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/nexodus-io/nexodus/internal/email"
	"github.com/nexodus-io/nexodus/internal/models"
	htmltemplate "html/template"
	texttemplate "text/template"
	"time"
)

//go:embed templates/invitation.html
var invitationHtml string
var invitationHtmlTemplate *htmltemplate.Template

//go:embed templates/invitation.txt
var invitationText string
var invitationTextTemplate *texttemplate.Template

//go:embed templates/nexodus.png
var nexodusPng []byte

func init() {
	var err error
	invitationHtmlTemplate, err = htmltemplate.New("templates/invitation.html").Parse(invitationHtml)
	if err != nil {
		panic(err)
	}
	invitationTextTemplate, err = texttemplate.New("templates/invitation.txt").Parse(invitationText)
	if err != nil {
		panic(err)
	}
}

func (api *API) sendInvitationEmail(fromName string, to string, invitation *models.Invitation, orgName string) error {
	if api.SmtpServer.HostPort == "" {
		return nil
	}
	message, err := api.composeInvitationEmail(fromName, to, invitation, orgName)
	if err != nil {
		return err
	}
	return email.Send(api.SmtpServer, message)
}

func (api *API) composeInvitationEmail(fromName string, to string, invitation *models.Invitation, orgName string) (email.Message, error) {
	variables := struct {
		OrganizationName string
		FromUserName     string
		From             string
		Subject          string
		InvitationURL    string
		ExpiresIn        string
	}{
		InvitationURL:    fmt.Sprintf("%s#/invitations/%s/show", api.URL, invitation.ID),
		OrganizationName: orgName,
		Subject:          fmt.Sprintf("%s invited you to join %s", fromName, orgName),
		From:             fmt.Sprintf("%s <%s>", fromName, api.SmtpFrom),
		FromUserName:     fromName,
	}

	if !invitation.ExpiresAt.IsZero() {
		variables.ExpiresIn = humanize.Time(invitation.ExpiresAt.Add(30 * time.Second))
	}

	html := bytes.NewBuffer(nil)
	err := invitationHtmlTemplate.Execute(html, variables)
	if err != nil {
		return email.Message{}, err
	}

	text := bytes.NewBuffer(nil)
	err = invitationTextTemplate.Execute(text, variables)
	if err != nil {
		return email.Message{}, err
	}

	message := email.Message{
		From:         api.SmtpFrom,
		To:           []string{to},
		Subject:      variables.Subject,
		PlainMessage: text.String(),
		HtmlMessages: html.String(),
		Attachments: []email.Attachment{
			{
				Name:        "nexodus.png",
				ContentType: "image/png",
				Content:     bytes.NewReader(nexodusPng),
				Inline:      true,
			},
		},
	}
	return message, nil
}
