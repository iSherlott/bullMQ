package app

import (
	"context"

	bullmq "github.com/iSherlott/bullMQ"
)

func Run(ctx context.Context) error {
	redisConfig, consumeConfig, err := LoadConsumeConfig()
	if err != nil {
		return err
	}

	_, consumer := bullmq.New(redisConfig)
	return consumer.Consume(ctx, consumeConfig.QueueName, HandleJob, consumeConfig.Options)
}
