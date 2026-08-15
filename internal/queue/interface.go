package queue

import "context"

// JudgeQueue is the interface for the judge submission queue.
//
// Contract:
//   - Enqueue adds a submission ID with the given priority (lower = higher urgency).
//   - Dequeue BLOCKS until an item is available or ctx is cancelled.
//     It MUST NOT return ("", nil); callers rely on this to avoid busy-looping.
//     On context cancellation it returns ("", ctx.Err()).
//   - Len returns the current queue depth (approximate for Redis).
//   - Close releases resources; subsequent Dequeue calls may return an error.
type JudgeQueue interface {
	Enqueue(ctx context.Context, submissionID string, priority int) error
	Dequeue(ctx context.Context) (string, error)
	Len() int
	Close() error
}
