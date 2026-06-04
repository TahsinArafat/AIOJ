package queue

import "context"

type JudgeQueue interface {
	Enqueue(ctx context.Context, submissionID string, priority int) error
	Dequeue(ctx context.Context) (string, error)
	Len() int
	Close() error
}
