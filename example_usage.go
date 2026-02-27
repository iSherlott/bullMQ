package bullmq

import (
	"context"
	"encoding/json"
	"log"
)

type sampleMessage struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

func ExampleUsage() {
	// Production: set environment variables (REDIS_ADDR/REDIS_HOST, etc.).
	// Local/test: you may keep a `.env` file; LoadConfigFromEnv will load it when present.
	config, err := LoadConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	publisher, consumer := New(config)

	ctx := context.Background()
	queueToPublish := "notification_queue"
	queueToConsume := "notification_queue"

	payload, err := json.Marshal(sampleMessage{
		UserID:  "user_123",
		Message: "hello bullmq",
	})
	if err != nil {
		log.Fatal(err)
	}

	_, err = publisher.Publish(ctx, queueToPublish, "send_notification", payload)
	if err != nil {
		log.Fatal(err)
	}

	err = consumer.Consume(ctx, queueToConsume, func(_ context.Context, job Job) error {
		var message sampleMessage
		if unmarshalErr := json.Unmarshal(job.Payload, &message); unmarshalErr != nil {
			return unmarshalErr
		}

		log.Printf("processing queue=%s job_id=%s user_id=%s", job.QueueName, job.ID, message.UserID)
		return nil
	}, ConsumerOptions{
		RetryLimit:      3,
		DeadLetterQueue: "notification_queue_dead_letter",
	})
	if err != nil {
		log.Fatal(err)
	}
}
