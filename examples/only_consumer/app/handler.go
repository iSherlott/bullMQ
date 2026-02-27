package app

import (
	"context"
	"log"

	bullmq "github.com/iSherlott/bullMQ"
)

type Message struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

func HandleJob(_ context.Context, job bullmq.Job) error {
	var message Message
	if err := job.UnmarshalPayload(&message); err != nil {
		return err
	}

	log.Printf("consumed queue=%s job_id=%s name=%s payload=%s user_id=%s message=%s", job.QueueName, job.ID, job.Name, job.PayloadString(), message.UserID, message.Message)
	return nil
}
