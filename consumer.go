package bullmq

import (
	"context"
	"errors"
	"strconv"
	"time"
)

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (consumer *Consumer) Consume(ctx context.Context, queueName string, handler JobHandler, options ConsumerOptions) error {
	if queueName == "" {
		return errors.New("queue_name is required")
	}
	if handler == nil {
		return errors.New("handler is required")
	}

	retryLimit := options.retryLimit()
	if retryLimit < 0 {
		return errors.New("retry_limit cannot be negative")
	}

	deadLetterQueueName := options.deadLetterQueue(queueName)

	// Normalize legacy state keys so BRPOP/LPUSH won't crash with WRONGTYPE.
	if err := consumer.store.EnsureQueueSchema(ctx, queueName); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		jobID, err := consumer.store.PopWaiting(ctx, queueName)
		if err != nil {
			return err
		}

		if err := consumer.store.MarkProcessing(ctx, queueName, jobID); err != nil {
			return err
		}

		jobData, err := consumer.store.GetJobHash(ctx, queueName, jobID)
		if err != nil {
			return err
		}

		if len(jobData) == 0 {
			if unmarkErr := consumer.store.UnmarkProcessing(ctx, queueName, jobID); unmarkErr != nil {
				return unmarkErr
			}
			continue
		}

		createdAt := time.Now().UTC()
		if ts, ok := jobData["timestamp"]; ok {
			if milliseconds, parseErr := strconv.ParseInt(ts, 10, 64); parseErr == nil {
				createdAt = time.UnixMilli(milliseconds).UTC()
			}
		}

		job := Job{
			ID:        jobID,
			QueueName: queueName,
			Name:      jobData["name"],
			Payload:   []byte(jobData["data"]),
			CreatedAt: createdAt,
		}

		handlerErr := handler(ctx, job)
		if handlerErr == nil {
			if err := consumer.store.MarkCompleted(ctx, queueName, jobID); err != nil {
				return err
			}
			continue
		}

		var waitingErr *WaitingError
		if errors.As(handlerErr, &waitingErr) {
			if err := consumer.store.UnmarkProcessing(ctx, queueName, jobID); err != nil {
				return err
			}
			if err := consumer.store.RequeueWaiting(ctx, queueName, jobID); err != nil {
				return err
			}
			continue
		}

		var waitingChildrenErr *WaitingChildrenError
		if errors.As(handlerErr, &waitingChildrenErr) {
			if err := consumer.store.UnmarkProcessing(ctx, queueName, jobID); err != nil {
				return err
			}
			if err := consumer.store.RequeueWaiting(ctx, queueName, jobID); err != nil {
				return err
			}
			continue
		}

		var rateLimitErr *RateLimitError
		if errors.As(handlerErr, &rateLimitErr) {
			if err := consumer.store.UnmarkProcessing(ctx, queueName, jobID); err != nil {
				return err
			}
			if err := sleepWithContext(ctx, rateLimitErr.Delay); err != nil {
				return err
			}
			if err := consumer.store.RequeueWaiting(ctx, queueName, jobID); err != nil {
				return err
			}
			continue
		}

		var delayedErr *DelayedError
		if errors.As(handlerErr, &delayedErr) {
			if err := consumer.store.UnmarkProcessing(ctx, queueName, jobID); err != nil {
				return err
			}
			if err := sleepWithContext(ctx, delayedErr.Delay); err != nil {
				return err
			}
			if err := consumer.store.RequeueWaiting(ctx, queueName, jobID); err != nil {
				return err
			}
			continue
		}

		var unrecoverableErr *UnrecoverableError
		isUnrecoverable := errors.As(handlerErr, &unrecoverableErr)

		attempts, err := consumer.store.IncrementAttempts(ctx, queueName, jobID)
		if err != nil {
			return err
		}

		if err := consumer.store.UnmarkProcessing(ctx, queueName, jobID); err != nil {
			return err
		}

		if err := consumer.store.UpdateLastError(ctx, queueName, jobID, handlerErr.Error()); err != nil {
			return err
		}

		if !isUnrecoverable && attempts <= retryLimit {
			if err := consumer.store.RequeueWaiting(ctx, queueName, jobID); err != nil {
				return err
			}
			continue
		}

		// Dead-letter publish keeps the original job hash intact.
		if _, err := consumer.store.Publish(ctx, deadLetterQueueName, job.Name, job.Payload); err != nil {
			return err
		}

		if err := consumer.store.MarkFailed(ctx, queueName, jobID, handlerErr.Error()); err != nil {
			return err
		}
	}
}
