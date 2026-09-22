package mail

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type SMTPSender struct{ cfg SMTPConfig }

func NewSMTPSender(cfg SMTPConfig) *SMTPSender { return &SMTPSender{cfg: cfg} }

func (s *SMTPSender) Send(_ context.Context, msg *Message) error {
	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprintf("%d", s.cfg.Port))
	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}
	from := msg.From
	if from == "" {
		from = s.cfg.From
	}
	body := buildPlain(from, msg)
	return smtp.SendMail(addr, auth, from, msg.To, []byte(body))
}

func buildPlain(from string, msg *Message) string {
	var b strings.Builder
	for k, v := range msg.Headers {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteString("\r\n")
	}
	b.WriteString("From: ")
	b.WriteString(from)
	b.WriteString("\r\n")
	b.WriteString("To: ")
	b.WriteString(strings.Join(msg.To, ", "))
	b.WriteString("\r\n")
	if len(msg.Cc) > 0 {
		b.WriteString("Cc: ")
		b.WriteString(strings.Join(msg.Cc, ", "))
		b.WriteString("\r\n")
	}
	b.WriteString("Subject: ")
	b.WriteString(msg.Subject)
	b.WriteString("\r\n")
	b.WriteString("Date: ")
	b.WriteString(time.Now().Format(time.RFC1123Z))
	b.WriteString("\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(msg.Body)
	return b.String()
}
