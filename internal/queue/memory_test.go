package queue

import (
	"context"
	"testing"
	"time"
)

func TestQueue(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	m.Enqueue(ctx, "a", 1)
	m.Enqueue(ctx, "b", 1)
	if m.Len() != 2 {
		t.Fatalf("len: %d", m.Len())
	}
	id, _ := m.Dequeue(ctx)
	if id != "a" {
		t.Fatalf("got %s", id)
	}
	id, _ = m.Dequeue(ctx)
	if id != "b" {
		t.Fatalf("got %s", id)
	}
}

func TestBlockingDequeue(t *testing.T) {
	m := NewMemory()
	done := make(chan string)
	go func() { id, _ := m.Dequeue(context.Background()); done <- id }()
	time.Sleep(50 * time.Millisecond)
	m.Enqueue(context.Background(), "x", 1)
	select {
	case id := <-done:
		if id != "x" {
			t.Fatalf("got %s", id)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
