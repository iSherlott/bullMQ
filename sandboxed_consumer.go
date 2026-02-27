package bullmq

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/iSherlott/bullMQ/sandbox"
)

// SandboxedProcessOptions configures running job processing via an external process.
//
// The child process must speak the JSON protocol implemented by `sandbox.RunChild`.
type SandboxedProcessOptions struct {
	// ProcessFile is the child binary path.
	ProcessFile string

	// MainFile is optional. If provided, the child is started as: MainFile ProcessFile
	// This mirrors BullMQ's "mainFile" concept.
	MainFile string

	Dir         string
	Env         []string
	KillTimeout time.Duration

	// Token is an optional worker token passed to the child.
	Token string
}

// ConsumeSandboxed consumes jobs using a sandboxed processor (external process).
//
// It preserves job hashes and only moves state (waiting -> processing -> completed/failed),
// matching the library's non-deleting semantics.
func (consumer *Consumer) ConsumeSandboxed(ctx context.Context, queueName string, sandboxOpts SandboxedProcessOptions, options ConsumerOptions) error {
	if queueName == "" {
		return errors.New("queue_name is required")
	}
	if sandboxOpts.ProcessFile == "" {
		return errors.New("process_file is required")
	}

	retryLimit := options.retryLimit()
	if retryLimit < 0 {
		return errors.New("retry_limit cannot be negative")
	}
	deadLetterQueueName := options.deadLetterQueue(queueName)

	if err := consumer.store.EnsureQueueSchema(ctx, queueName); err != nil {
		return err
	}

	pool := sandbox.NewChildPool(sandbox.ChildPoolOpts{
		MainFile: sandboxOpts.MainFile,
		SandboxedOptions: sandbox.SandboxedOptions{
			Dir:         sandboxOpts.Dir,
			Env:         sandboxOpts.Env,
			KillTimeout: sandboxOpts.KillTimeout,
		},
	})
	defer func() { _ = pool.Clean(context.Background()) }()

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

		ts := time.Now().UTC().UnixMilli()
		if tsStr, ok := jobData["timestamp"]; ok {
			if milliseconds, parseErr := strconv.ParseInt(tsStr, 10, 64); parseErr == nil {
				ts = milliseconds
			}
		}

		jobJson := sandbox.JobJsonSandbox{
			ID:        jobID,
			QueueName: queueName,
			Name:      jobData["name"],
			Data:      json.RawMessage(jobData["data"]),
			Timestamp: ts,
		}

		child, err := pool.Retain(ctx, sandboxOpts.ProcessFile)
		if err != nil {
			return err
		}

		_, sendErr := child.Send(ctx, jobJson, sandboxOpts.Token)
		if sendErr != nil {
			// If the child crashed or got corrupted, remove it from the pool.
			pool.Remove(child)
			stopCtx, cancel := context.WithTimeout(context.Background(), effectiveKillTimeout(sandboxOpts.KillTimeout))
			_ = child.Stop(stopCtx)
			cancel()
		} else {
			pool.Release(child)
		}

		handlerErr := decodeSandboxError(sendErr)
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
		payload := []byte(jobData["data"])
		jobName := jobData["name"]
		if _, err := consumer.store.Publish(ctx, deadLetterQueueName, jobName, payload); err != nil {
			return err
		}

		if err := consumer.store.MarkFailed(ctx, queueName, jobID, handlerErr.Error()); err != nil {
			return err
		}
	}
}

func effectiveKillTimeout(d time.Duration) time.Duration {
	if d <= 0 {
		return 5 * time.Second
	}
	return d
}

// decodeSandboxError maps child error strings to BullMQ-style typed errors.
//
// Supported formats:
// - "<CODE>"
// - "<CODE>: message"
// - "<CODE>|<delay_ms>|message"
//
// Where CODE is one of the exported constants in errors.go.
func decodeSandboxError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()
	code, delay, detail, ok := parseBullmqCode(msg)
	if !ok {
		return err
	}

	switch code {
	case WAITING_ERROR:
		return &WaitingError{Message: detail}
	case WAITING_CHILDREN_ERROR:
		return &WaitingChildrenError{Message: detail}
	case UNRECOVERABLE_ERROR:
		return &UnrecoverableError{Message: detail}
	case RATE_LIMIT_ERROR:
		return &RateLimitError{Delay: delay, Message: detail}
	case DELAYED_ERROR:
		return &DelayedError{Delay: delay, Message: detail}
	default:
		return err
	}
}

func parseBullmqCode(s string) (code string, delay time.Duration, detail string, ok bool) {
	for _, c := range []string{WAITING_ERROR, WAITING_CHILDREN_ERROR, UNRECOVERABLE_ERROR, RATE_LIMIT_ERROR, DELAYED_ERROR} {
		if strings.HasPrefix(s, c) {
			code = c
			detail = strings.TrimSpace(strings.TrimPrefix(s, c))
			if strings.HasPrefix(detail, ":") {
				detail = strings.TrimSpace(strings.TrimPrefix(detail, ":"))
				return code, 0, detail, true
			}

			// Optional "|<ms>|..." format.
			if strings.HasPrefix(detail, "|") {
				rest := strings.TrimPrefix(detail, "|")
				parts := strings.SplitN(rest, "|", 2)
				if len(parts) >= 1 {
					if ms, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64); err == nil {
						delay = time.Duration(ms) * time.Millisecond
					}
				}
				if len(parts) == 2 {
					detail = strings.TrimSpace(parts[1])
				} else {
					detail = ""
				}
				return code, delay, detail, true
			}

			return code, 0, strings.TrimSpace(detail), true
		}
	}
	return "", 0, "", false
}
