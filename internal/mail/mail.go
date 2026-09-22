// Package mail provides a swappable mail-sending backend.
package mail

import "context"

type Message struct {
	From    string
	To      []string
	Cc      []string
	Subject string
	Body    string
	IsHTML  bool
	Headers map[string]string
}

type Sender interface {
	Send(ctx context.Context, msg *Message) error
}

// NoopSender is used when mail delivery is disabled.
type NoopSender struct{}

func (NoopSender) Send(_ context.Context, _ *Message) error { return nil }
