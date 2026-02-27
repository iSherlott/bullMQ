package bullmq

import "time"

const (
	WAITING_ERROR          = "bullmq:movedToWait"
	WAITING_CHILDREN_ERROR = "bullmq:movedToWaitingChildren"
	UNRECOVERABLE_ERROR    = "bullmq:unrecoverable"
	RATE_LIMIT_ERROR       = "bullmq:rateLimitExceeded"
	DELAYED_ERROR          = "bullmq:movedToDelayed"
)

// WaitingError is returned by a handler to move a job back to waiting
// without counting as a failed attempt.
type WaitingError struct {
	Message string
}

func (e *WaitingError) Error() string {
	if e == nil {
		return WAITING_ERROR
	}
	if e.Message == "" {
		return WAITING_ERROR
	}
	return WAITING_ERROR + ": " + e.Message
}

// WaitingChildrenError is returned by a handler to move a job back to waiting
// because it depends on children/other prerequisites.
type WaitingChildrenError struct {
	Message string
}

func (e *WaitingChildrenError) Error() string {
	if e == nil {
		return WAITING_CHILDREN_ERROR
	}
	if e.Message == "" {
		return WAITING_CHILDREN_ERROR
	}
	return WAITING_CHILDREN_ERROR + ": " + e.Message
}

// UnrecoverableError forces the job to be failed/dead-lettered even if
// attemptsMade are lower than the expected retry limit.
type UnrecoverableError struct {
	Message string
}

func (e *UnrecoverableError) Error() string {
	if e == nil {
		return UNRECOVERABLE_ERROR
	}
	if e.Message == "" {
		return UNRECOVERABLE_ERROR
	}
	return UNRECOVERABLE_ERROR + ": " + e.Message
}

// RateLimitError indicates the queue has reached a rate limit.
// The consumer should requeue the job, optionally after waiting Delay.
type RateLimitError struct {
	Delay   time.Duration
	Message string
}

func (e *RateLimitError) Error() string {
	if e == nil {
		return RATE_LIMIT_ERROR
	}
	if e.Message == "" {
		return RATE_LIMIT_ERROR
	}
	return RATE_LIMIT_ERROR + ": " + e.Message
}

// DelayedError indicates the job should be delayed and retried later.
// The consumer should requeue the job after waiting Delay.
type DelayedError struct {
	Delay   time.Duration
	Message string
}

func (e *DelayedError) Error() string {
	if e == nil {
		return DELAYED_ERROR
	}
	if e.Message == "" {
		return DELAYED_ERROR
	}
	return DELAYED_ERROR + ": " + e.Message
}
