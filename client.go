package bullmq

import (
	"context"

	"github.com/iSherlott/bullMQ/redis"
)

type Publisher struct {
	store *redis.Store
}

type Consumer struct {
	store *redis.Store
}

func New(config Config) (*Publisher, *Consumer) {
	normalizedConfig := config.normalize()
	store := redis.NewStore(redis.Options{
		Addr:          normalizedConfig.Addr,
		Password:      normalizedConfig.Password,
		DB:            normalizedConfig.DB,
		KeyPrefix:     normalizedConfig.KeyPrefix,
		UseTLS:        normalizedConfig.UseTLS,
		TLSServerName: normalizedConfig.Host,
	})

	publisher := &Publisher{store: store}
	consumer := &Consumer{store: store}

	return publisher, consumer
}

func (publisher *Publisher) Publish(ctx context.Context, queueName string, jobName string, payload []byte) (string, error) {
	return publisher.store.Publish(ctx, queueName, jobName, payload)
}
