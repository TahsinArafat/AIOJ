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

func (q *RedisQueue) Enqueue(ctx context.Context, id string) error {
	return q.client.LPush(ctx, q.queueName, id).Err()
}

func (q *RedisQueue) Dequeue(ctx context.Context) (string, error) {
	result, err := q.client.BRPop(ctx, 5*time.Second, q.queueName).Result()
	if err != nil {
		return "", err
	}
	return result[1], nil
}

func (q *RedisQueue) Len() int {
	l, err := q.client.LLen(context.Background(), q.queueName).Result()
	if err != nil {
		return 0
	}
	return int(l)
}

func (q *RedisQueue) Close() error {
	return q.client.Close()
}
