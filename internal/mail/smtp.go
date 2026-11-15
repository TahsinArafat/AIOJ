package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// defaultTimeout bounds every dial and every read/write on the SMTP
// conversation. A transactional provider that is slow or unreachable must not
// pin the requesting goroutine indefinitely.
const defaultTimeout = 10 * time.Second

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// Timeout bounds the whole exchange. Zero means defaultTimeout.
	Timeout time.Duration
}

type SMTPSender struct{ cfg SMTPConfig }

func NewSMTPSender(cfg SMTPConfig) *SMTPSender { return &SMTPSender{cfg: cfg} }

func (s *SMTPSender) Send(ctx context.Context, msg *Message) error {
	if len(msg.To) == 0 {
		return fmt.Errorf("mail: no recipients")
	}
	if s.cfg.Host == "" {
		return fmt.Errorf("mail: smtp host is not configured")
	}

	from := msg.From
	if from == "" {
		from = s.cfg.From
	}

	timeout := s.cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Port 465 is implicit TLS: TLS starts immediately, before any SMTP
	// greeting. smtp.SendMail cannot do this, which is why the connection is
	// built by hand here.
	implicitTLS := usesImplicitTLS(s.cfg.Port)

	dialer := &net.Dialer{Timeout: timeout}
	var (
		conn net.Conn
		err  error
	)
	if implicitTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", s.addr(), &tls.Config{ServerName: s.cfg.Host})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", s.addr())
	}
	if err != nil {
		return fmt.Errorf("mail: dial %s: %w", s.addr(), err)
	}

	// The deadline above bounds the timeout; this watchdog is what makes an
	// explicit cancellation interrupt a read that is already blocked, since
	// closing the connection is the only way to unblock one.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-stop:
		}
	}()

	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("mail: smtp handshake: %w", err)
	}
	defer c.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	if err := s.authenticate(c, implicitTLS); err != nil {
		return err
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("mail: MAIL FROM <%s>: %w", from, err)
	}
	for _, to := range msg.To {
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("mail: RCPT TO <%s>: %w", to, err)
		}
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mail: DATA: %w", err)
	}
	if _, err := w.Write([]byte(buildPlain(from, msg))); err != nil {
		return fmt.Errorf("mail: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: finish body: %w", err)
	}
	return c.Quit()
}

func (s *SMTPSender) addr() string {
	return net.JoinHostPort(s.cfg.Host, fmt.Sprintf("%d", s.cfg.Port))
}

// usesImplicitTLS reports whether the port requires TLS before the SMTP
// greeting (SMTPS). Every other port connects in the clear and is upgraded with
// STARTTLS.
func usesImplicitTLS(port int) bool { return port == 465 }

// authenticate upgrades to TLS when the server offers it, then sends
// credentials. net/smtp does not upgrade on its own, so a plaintext
// conversation would otherwise carry the password.
//
// A connection that is already TLS (port 465) advertises no STARTTLS, so the
// extension check is skipped for it.
func (s *SMTPSender) authenticate(c *smtp.Client, implicitTLS bool) error {
	if s.cfg.Username == "" {
		return nil
	}

	if !implicitTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: s.cfg.Host}); err != nil {
				return fmt.Errorf("mail: STARTTLS: %w", err)
			}
		} else {
			return fmt.Errorf("mail: server %s offers no STARTTLS — refusing to send "+
				"credentials over a cleartext connection (use port 465 for implicit TLS)", s.cfg.Host)
		}
	}

	if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
		return fmt.Errorf("mail: auth as %s: %w", s.cfg.Username, err)
	}
	return nil
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
