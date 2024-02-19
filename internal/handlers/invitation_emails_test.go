package handlers

import (
	"bytes"
	"crypto/tls"
	_ "embed"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/email"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/stretchr/testify/require"
	"math/rand"
	"os"
	"testing"
	"time"
)

//go:embed invitation_emails_test.fixture
var expectedEmail string

func TestAPI_composeInvitationEmail(t *testing.T) {
	require := require.New(t)
	api := &API{
		FrontendURL: "https://try.nexodus.io",
		SmtpFrom:    "no-reply@amazonses.com",
	}
	invite := &models.Invitation{
		Base: models.Base{
			ID: uuid.MustParse("8829327e-99a5-4438-9ad5-f4a552e42a0b"),
		},
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	message, err := api.composeInvitationEmail("hchirino", `chirino@gmail.com`, invite, "test org")
	require.NoError(err)
	// #nosec G404
	message.Rand = rand.New(rand.NewSource(0))

	buf := bytes.NewBuffer(nil)
	err = message.Write(buf)
	require.NoError(err)

	// Uncomment to update the fixture:
	//os.WriteFile("invitation_emails_test.fixture", buf.Bytes(), 0644)
	require.Equal(expectedEmail, buf.String())
}

func TestAPI_sendInvitationEmail(t *testing.T) {
	if os.Getenv("NEXAPI_SMTP_HOST_PORT") == "" {
		t.SkipNow()
	}
	api := &API{
		FrontendURL: "https://try.nexodus.io",
		SmtpServer: email.SmtpServer{
			HostPort: os.Getenv("NEXAPI_SMTP_HOST_PORT"),
			User:     os.Getenv("NEXAPI_SMTP_USER"),
			Password: os.Getenv("NEXAPI_SMTP_PASSWORD"),
		},
		SmtpFrom: os.Getenv("NEXAPI_SMTP_FROM"),
	}
	if os.Getenv("NEXAPI_SMTP_TLS") == "true" {
		// #nosec G402
		api.SmtpServer.Tls = &tls.Config{}
	}
	invite := &models.Invitation{
		Base: models.Base{
			ID: uuid.MustParse("8829327e-99a5-4438-9ad5-f4a552e42a0b"),
		},
		ExpiresAt: time.Now().Add(5 * 24 * time.Hour),
	}
	err := api.sendInvitationEmail("hchirino", `chirino@gmail.com`, invite, "Lab DMZ")
	require.NoError(t, err)
}
