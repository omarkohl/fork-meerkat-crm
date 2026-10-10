package services

import (
	"strings"
	"testing"

	"meerkat/config"

	"github.com/stretchr/testify/assert"
)

func TestSendEmailNoOpWithoutChannelOrRecipient(t *testing.T) {
	assert.NoError(t, SendEmail(config.Config{}, EmailMessage{To: "a@example.com", Subject: "s", HTML: "<p>x</p>"}))
	assert.NoError(t, SendEmail(config.Config{UseSMTP: true}, EmailMessage{To: "", Subject: "s"}))
}

func TestSendEmailRefusesHeaderInjection(t *testing.T) {
	cfg := config.Config{UseSMTP: true, SMTPHost: "127.0.0.1", SMTPPort: 1, SMTPFromEmail: "from@example.com"}
	err := SendEmail(cfg, EmailMessage{To: "victim@example.com\r\nBcc: attacker@example.com", Subject: "s"})
	assert.ErrorContains(t, err, "line break")
	err = SendEmail(cfg, EmailMessage{To: "a@example.com", Subject: "hi\r\nBcc: attacker@example.com"})
	assert.ErrorContains(t, err, "line break")
}

func TestSendEmailReportsAllChannelsFailing(t *testing.T) {
	// Nothing listens on port 1, so the SMTP channel fails.
	cfg := config.Config{UseSMTP: true, SMTPHost: "127.0.0.1", SMTPPort: 1, SMTPFromEmail: "from@example.com"}
	err := SendEmail(cfg, EmailMessage{To: "a@example.com", Subject: "s", HTML: "x"})
	assert.ErrorContains(t, err, "all email channels failed")
}

func TestBuildSMTPMessage(t *testing.T) {
	msg := string(buildSMTPMessage("from@example.com", EmailMessage{To: "to@example.com", Subject: "Grüße: Geburtstag", HTML: "<p>Hi</p>"}))
	head, body, found := strings.Cut(msg, "\r\n\r\n")
	assert.True(t, found, "headers and body are separated by an empty line")
	assert.Equal(t, "<p>Hi</p>", body)
	assert.Contains(t, head, "From: from@example.com\r\n")
	assert.Contains(t, head, "To: to@example.com\r\n")
	assert.Contains(t, head, "MIME-Version: 1.0")
	assert.Contains(t, head, `Content-Type: text/html; charset="UTF-8"`)
	assert.Contains(t, head, "Subject: =?UTF-8?q?Gr=C3=BC=C3=9Fe", "non-ASCII subjects are encoded")
}
