package bullmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type Job struct {
	ID        string
	QueueName string
	Name      string
	Payload   []byte
	CreatedAt time.Time
}

func (job Job) GetID() string {
	return job.ID
}

func (job Job) PayloadString() string {
	return string(job.Payload)
}

func (job Job) UnmarshalPayload(target any) error {
	return json.Unmarshal(job.Payload, target)
}

func (job Job) String() string {
	return fmt.Sprintf("Job{ID:%s QueueName:%s Name:%s Payload:%s CreatedAt:%s}", job.ID, job.QueueName, job.Name, job.PayloadString(), job.CreatedAt.UTC().Format(time.RFC3339Nano))
}

type JobHandler func(ctx context.Context, job Job) error

type ConsumerOptions struct {
	RetryLimit      int
	DeadLetterQueue string
}

func (options ConsumerOptions) retryLimit() int {
	return options.RetryLimit
}

func (options ConsumerOptions) deadLetterQueue(baseQueueName string) string {
	if options.DeadLetterQueue != "" {
		return options.DeadLetterQueue
	}

	return baseQueueName + "_dead_letter"
}
