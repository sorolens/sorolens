package reportdigest

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
	"strings"
)

// SMTPMailer sends digests over SMTP. It is configured from the environment
// so no code change is needed to point at a real server:
//
//	SMTP_ADDR  host:port of the SMTP server (e.g. smtp.example.com:587)
//	SMTP_FROM  the From address
//	SMTP_USER  optional PLAIN-auth username
//	SMTP_PASS  optional PLAIN-auth password
type SMTPMailer struct {
	Addr string
	From string
	Auth smtp.Auth
}

// NewSMTPMailerFromEnv builds an SMTPMailer, or returns (nil, false) when
// SMTP_ADDR/SMTP_FROM are not set so the caller can skip sending in
// environments without mail configured.
func NewSMTPMailerFromEnv() (*SMTPMailer, bool) {
	addr := os.Getenv("SMTP_ADDR")
	from := os.Getenv("SMTP_FROM")
	if addr == "" || from == "" {
		return nil, false
	}
	m := &SMTPMailer{Addr: addr, From: from}
	if user := os.Getenv("SMTP_USER"); user != "" {
		host := addr
		if i := strings.LastIndex(addr, ":"); i >= 0 {
			host = addr[:i]
		}
		m.Auth = smtp.PlainAuth("", user, os.Getenv("SMTP_PASS"), host)
	}
	return m, true
}

// Send delivers a multipart/alternative message with both text and HTML parts.
func (m *SMTPMailer) Send(_ context.Context, to, subject, htmlBody, textBody string) error {
	const boundary = "sorolens-digest-boundary"
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", m.From)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", boundary, textBody)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s\r\n", boundary, htmlBody)
	fmt.Fprintf(&b, "--%s--\r\n", boundary)

	return smtp.SendMail(m.Addr, m.Auth, m.From, []string{to}, []byte(b.String()))
}
