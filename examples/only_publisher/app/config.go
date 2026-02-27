package app

import bullmq "github.com/iSherlott/bullMQ"

type PublishConfig struct {
	QueueName string
	JobName   string
}

func LoadPublishConfig() (bullmq.Config, PublishConfig, error) {
	redisConfig, err := bullmq.LoadConfigFromEnv(".env")
	if err != nil {
		return bullmq.Config{}, PublishConfig{}, err
	}

	publishConfig := PublishConfig{
		QueueName: bullmq.EnvString("BULLMQ_QUEUE_NAME", "notification_queue"),
		JobName:   "send_notification",
	}

	return redisConfig, publishConfig, nil
}
