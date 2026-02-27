package app

import (
	"context"

	bullmq "github.com/iSherlott/bullMQ"
)

func Run(ctx context.Context) (string, error) {
	redisConfig, publishConfig, err := LoadPublishConfig()
	if err != nil {
		return "", err
	}

	publisher, _ := bullmq.New(redisConfig)

	payload, err := BuildPayload()
	if err != nil {
		return "", err
	}

	return publisher.Publish(ctx, publishConfig.QueueName, publishConfig.JobName, payload)
}
