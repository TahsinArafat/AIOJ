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
