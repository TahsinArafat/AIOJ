package queue

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultQueueName = "aioj:submissions"

type RedisQueue struct {
	client    *redis.Client
	queueName string
}

func NewRedisQueue(client *redis.Client) *RedisQueue {
	return &RedisQueue{
		client:    client,
		queueName: defaultQueueName,
	}
}

func (q *RedisQueue) Enqueue(ctx context.Context, id string, priority int) error {
	return q.client.ZAdd(ctx, q.queueName, redis.Z{
		Score:  float64(priority),
		Member: id,
	}).Err()
}

// Dequeue blocks until an item is available, the context is cancelled, or
// an unexpected Redis error occurs.
//
// BZPopMin already blocks server-side for up to `blockTimeout`; we loop on
// redis.Nil (timeout with no item) so the caller never sees ("", nil) — which
// would require it to busy-loop.  On context cancellation we propagate the
// context error so callers can clean up correctly.
func (q *RedisQueue) Dequeue(ctx context.Context) (string, error) {
	const blockTimeout = 2 * time.Second
	for {
		// Check context before each attempt.
		if err := ctx.Err(); err != nil {
			return "", err
		}

		result, err := q.client.BZPopMin(ctx, blockTimeout, q.queueName).Result()
		if err == nil {
			return result.Member.(string), nil
		}
		if err == redis.Nil {
			// BZPopMin timed out with no item — loop and try again.
			continue
		}
		// Propagate real errors (context cancelled, network, etc.).
		return "", err
	}
}

func (q *RedisQueue) Len() int {
	l, err := q.client.ZCard(context.Background(), q.queueName).Result()
	if err != nil {
		return 0
	}
	return int(l)
}

func (q *RedisQueue) Close() error {
	return q.client.Close()
}
