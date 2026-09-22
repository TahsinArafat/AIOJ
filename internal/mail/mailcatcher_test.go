package mail

import (
	"context"
	"testing"
)

func TestMailCatcher_SendAndRetrieve(t *testing.T) {
	c := NewMailCatcher()
	ctx := context.Background()

	msg := &Message{
		From: "noreply@aioj.com", To: []string{"alice@example.com"},
		Subject: "Welcome to AIOJ", Body: "Hello Alice",
	}
	if err := c.Send(ctx, msg); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	msgs := c.All()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Subject != "Welcome to AIOJ" {
		t.Errorf("got subject %q, want %q", msgs[0].Subject, "Welcome to AIOJ")
	}
	if msgs[0].To[0] != "alice@example.com" {
		t.Errorf("got To %q, want %q", msgs[0].To[0], "alice@example.com")
	}
}

func TestMailCatcher_FindByRecipient(t *testing.T) {
	c := NewMailCatcher()
	_ = c.Send(context.Background(), &Message{To: []string{"a@x.com"}, Subject: "to a"})
	_ = c.Send(context.Background(), &Message{To: []string{"b@x.com"}, Subject: "to b"})
	found := c.FindTo("a@x.com")
	if len(found) != 1 || found[0].Subject != "to a" {
		t.Errorf("FindTo returned wrong messages: %+v", found)
	}
}

func TestMailCatcher_Clear(t *testing.T) {
	c := NewMailCatcher()
	_ = c.Send(context.Background(), &Message{To: []string{"a@x.com"}, Subject: "x"})
	c.Clear()
	if len(c.All()) != 0 {
		t.Error("Clear did not empty messages")
	}
}
