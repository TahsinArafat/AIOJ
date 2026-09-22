package mail

import (
	"context"
	"sync"
)

// MailCatcher is an in-memory Sender for development and tests.
type MailCatcher struct {
	mu   sync.RWMutex
	msgs []*Message
}

func NewMailCatcher() *MailCatcher { return &MailCatcher{} }

func (c *MailCatcher) Send(_ context.Context, msg *Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := *msg
	if msg.To != nil {
		cp.To = append([]string(nil), msg.To...)
	}
	c.msgs = append(c.msgs, &cp)
	return nil
}

func (c *MailCatcher) All() []*Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]*Message, len(c.msgs))
	for i, m := range c.msgs {
		cp := *m
		if m.To != nil {
			cp.To = append([]string(nil), m.To...)
		}
		out[i] = &cp
	}
	return out
}

func (c *MailCatcher) FindTo(addr string) []*Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []*Message
	for _, m := range c.msgs {
		for _, to := range m.To {
			if to == addr {
				cp := *m
				if m.To != nil {
					cp.To = append([]string(nil), m.To...)
				}
				out = append(out, &cp)
				break
			}
		}
	}
	return out
}

func (c *MailCatcher) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = nil
}
