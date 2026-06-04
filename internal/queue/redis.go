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

func (q *RedisQueue) Dequeue(ctx context.Context) (string, error) {
	result, err := q.client.BZPopMin(ctx, 2*time.Second, q.queueName).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}
	return result.Member.(string), nil
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
