package app

import bullmq "github.com/iSherlott/bullMQ"

type ConsumeConfig struct {
	QueueName string
	Options   bullmq.ConsumerOptions
}

func LoadConsumeConfig() (bullmq.Config, ConsumeConfig, error) {
	redisConfig, err := bullmq.LoadConfigFromEnv(".env")
	if err != nil {
		return bullmq.Config{}, ConsumeConfig{}, err
	}

	consumeConfig := ConsumeConfig{
		QueueName: bullmq.EnvString("BULLMQ_QUEUE_NAME", "notification_queue"),
		Options: bullmq.ConsumerOptions{
			RetryLimit:      bullmq.EnvInt("BULLMQ_RETRY_LIMIT", 3),
			DeadLetterQueue: bullmq.EnvString("BULLMQ_DEAD_LETTER_QUEUE", "notification_queue_dead_letter"),
		},
	}

	return redisConfig, consumeConfig, nil
}
