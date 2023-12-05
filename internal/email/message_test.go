package email

import (
	"bytes"
	_ "embed"
	"github.com/stretchr/testify/require"
	"math/rand"
	"os"
	"testing"
)

//go:embed message_test.fixture
var expectedEmail string

func TestMessageEncode(t *testing.T) {

	testPng, err := os.Open("test.png")
	require.NoError(t, err)
	defer testPng.Close()

	email := Message{
		From:         "test@appetite-for-adventure.com",
		To:           []string{"chirino@icloud.com"},
		Subject:      "test 2",
		PlainMessage: `hello world`,
		HtmlMessages: `<html><body><div>Hello <b>World</b><br></div><div><br></div><div><b><img style="max-width:100%" src="test.png"></b><br></div><div><br></div><div><b>nice!</b><br></div></body></html>`,
		Attachments: []Attachment{
			{
				Name:        "test.png",
				ContentType: "image/png",
				Content:     testPng,
				Inline:      true,
			},
		},
		// this allows the multipart boundary to be deterministic
		// #nosec G404
		Rand: rand.New(rand.NewSource(0)),
	}
	buf := bytes.NewBuffer(nil)
	err = email.Write(buf)
	require.NoError(t, err)

	// Uncomment to update the fixture:
	//os.WriteFile("message_test.fixture", buf.Bytes(), 0644)
	require.Equal(t, expectedEmail, buf.String())

}
