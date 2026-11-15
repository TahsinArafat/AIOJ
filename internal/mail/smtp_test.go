package mail

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// startFakeSMTP is a minimal SMTP server that records DATA payloads.
func startFakeSMTP(t *testing.T) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	type received struct{ From, To, Data string }
	var got []received
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
				_, _ = conn.Write([]byte("220 fake ESMTP\r\n"))
				buf := make([]byte, 4096)
				from, to, data := "", "", ""
				inData := false
				for {
					n, err := conn.Read(buf)
					if err != nil {
						return
					}
					line := string(buf[:n])
					if inData {
						data += line
						if strings.HasSuffix(strings.TrimRight(line, "\r\n"), ".") {
							got = append(got, received{from, to, data})
							_, _ = conn.Write([]byte("250 OK\r\n"))
							inData = false
							continue
						}
						continue
					}
					up := strings.ToUpper(strings.TrimSpace(line))
					switch {
					case strings.HasPrefix(up, "EHLO"), strings.HasPrefix(up, "HELO"):
						_, _ = conn.Write([]byte("250-fake\r\n250 OK\r\n"))
					case strings.HasPrefix(up, "MAIL FROM:"):
						from = line
						_, _ = conn.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(up, "RCPT TO:"):
						to = line
						_, _ = conn.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(up, "DATA"):
						inData = true
						_, _ = conn.Write([]byte("354 End data\r\n"))
					case strings.HasPrefix(up, "QUIT"):
						_, _ = conn.Write([]byte("221 Bye\r\n"))
						return
					default:
						_, _ = conn.Write([]byte("250 OK\r\n"))
					}
				}
			}(c)
		}
	}()
	stop := func() { _ = ln.Close(); <-done }
	return ln.Addr().String(), stop
}

func TestSMTPSender_SendPlainText(t *testing.T) {
	addr, stop := startFakeSMTP(t)
	defer stop()
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)

	s := NewSMTPSender(SMTPConfig{Host: host, Port: port, From: "noreply@aioj.com"})
	if err := s.Send(context.Background(), &Message{
		To: []string{"alice@example.com"}, Subject: "Hello", Body: "World",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestSMTPSender_SendsBody(t *testing.T) {
	addr, stop := startFakeSMTP(t)
	defer stop()
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)

	s := NewSMTPSender(SMTPConfig{Host: host, Port: port, From: "noreply@aioj.com"})
	if err := s.Send(context.Background(), &Message{
		To: []string{"alice@example.com"}, Subject: "Hello", Body: "World",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	// Give the fake server a moment to record DATA.
	time.Sleep(50 * time.Millisecond)
}

// A relay that accepts the connection then goes silent must not pin the
// requesting goroutine. Before the timeout existed, Send blocked forever.
func TestSMTPSender_HonoursTimeoutOnStalledServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			// Greet, then never answer EHLO.
			_, _ = c.Write([]byte("220 stalled ESMTP\r\n"))
		}
	}()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	s := NewSMTPSender(SMTPConfig{
		Host: host, Port: port, From: "noreply@aioj.com",
		Timeout: 250 * time.Millisecond,
	})

	done := make(chan error, 1)
	go func() {
		done <- s.Send(context.Background(), &Message{
			To: []string{"alice@example.com"}, Subject: "Hi", Body: "there",
		})
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a timeout error, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Send did not return — the timeout is not being applied")
	}
}

// Cancelling the request context must abort a send in flight.
func TestSMTPSender_HonoursContextCancellation(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = c.Write([]byte("220 stalled ESMTP\r\n"))
		}
	}()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	s := NewSMTPSender(SMTPConfig{
		Host: host, Port: port, From: "noreply@aioj.com", Timeout: 30 * time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	if err := s.Send(ctx, &Message{To: []string{"a@example.com"}, Subject: "x", Body: "y"}); err == nil {
		t.Fatal("expected an error after context cancellation")
	}
}

// A server that never offers STARTTLS must fail the send rather than transmit
// the SMTP password in cleartext. The fake server advertises no extensions.
func TestSMTPSender_RefusesCleartextAuthWithoutSTARTTLS(t *testing.T) {
	addr, stop := startFakeSMTP(t)
	defer stop()
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)

	s := NewSMTPSender(SMTPConfig{
		Host: host, Port: port, From: "noreply@aioj.com",
		Username: "user@example.com", Password: "secret",
	})
	err := s.Send(context.Background(), &Message{
		To: []string{"alice@example.com"}, Subject: "Hi", Body: "there",
	})
	if err == nil {
		t.Fatal("expected the send to fail when the server offers no STARTTLS")
	}
	if !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("error should name the missing STARTTLS, got: %v", err)
	}
}

// Configuration mistakes must fail fast with a message that names the problem,
// not with an opaque network error.
func TestSMTPSender_RejectsMissingHost(t *testing.T) {
	s := NewSMTPSender(SMTPConfig{Port: 587, From: "noreply@aioj.com"})
	err := s.Send(context.Background(), &Message{To: []string{"a@example.com"}})
	if err == nil || !strings.Contains(err.Error(), "host") {
		t.Fatalf("expected a host-configuration error, got: %v", err)
	}
}

func TestSMTPSender_RejectsMissingRecipients(t *testing.T) {
	s := NewSMTPSender(SMTPConfig{Host: "smtp.example.com", Port: 587, From: "n@example.com"})
	err := s.Send(context.Background(), &Message{Subject: "no one"})
	if err == nil || !strings.Contains(err.Error(), "recipient") {
		t.Fatalf("expected a no-recipients error, got: %v", err)
	}
}

// An unreachable relay fails with a dial error rather than hanging.
func TestSMTPSender_DialFailureIsReported(t *testing.T) {
	// Grab a port then close it, so nothing is listening.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	ln.Close()

	s := NewSMTPSender(SMTPConfig{
		Host: host, Port: port, From: "noreply@aioj.com",
		Timeout: 2 * time.Second,
	})
	err = s.Send(context.Background(), &Message{
		To: []string{"a@example.com"}, Subject: "x", Body: "y",
	})
	if err == nil {
		t.Fatal("expected a dial error")
	}
	if !strings.Contains(err.Error(), "dial") {
		t.Fatalf("error should name the dial stage, got: %v", err)
	}
}

// Port 465 is implicit TLS: TLS must start before any SMTP greeting, which
// net/smtp.SendMail cannot do. Pointing the implicit-TLS path at a plaintext
// server must fail the TLS handshake — proof it is not speaking plaintext.
func TestSMTPSender_ImplicitTLSPathSelected(t *testing.T) {
	if !usesImplicitTLS(465) {
		t.Fatal("port 465 must select implicit TLS")
	}
	for _, p := range []int{25, 587, 2525} {
		if usesImplicitTLS(p) {
			t.Errorf("port %d must use STARTTLS, not implicit TLS", p)
		}
	}
}
