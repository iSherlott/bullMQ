package bullmq

import (
	"errors"
	"math"
	"math/rand"
	"time"
)

// MinimalJob matches BullMQ's minimal-job concept for backoff strategies.
// It intentionally stays tiny so it can be implemented by multiple job types.
type MinimalJob interface {
	GetID() string
}

// BackoffOptions is a normalized backoff configuration.
type BackoffOptions struct {
	Type   string
	Delay  time.Duration
	Jitter float64
}

// BackoffStrategy returns a computed delay for the next retry.
// ok=false means "do not retry / no backoff".
type BackoffStrategy func(delay time.Duration, jitter float64) func(attemptsMade int, err error, job MinimalJob) (time.Duration, bool)

type BuiltInStrategies map[string]func(delay time.Duration, jitter float64) func(attemptsMade int, err error, job MinimalJob) (time.Duration, bool)

type Backoffs struct{}

// BuiltinStrategies mirrors BullMQ built-in backoff strategies.
var builtinStrategies BuiltInStrategies = BuiltInStrategies{
	"fixed": func(delay time.Duration, jitter float64) func(int, error, MinimalJob) (time.Duration, bool) {
		return func(_ int, _ error, _ MinimalJob) (time.Duration, bool) {
			return applyJitter(delay, jitter), true
		}
	},
	"exponential": func(delay time.Duration, jitter float64) func(int, error, MinimalJob) (time.Duration, bool) {
		return func(attemptsMade int, _ error, _ MinimalJob) (time.Duration, bool) {
			if attemptsMade <= 0 {
				attemptsMade = 1
			}
			scaled := time.Duration(float64(delay) * math.Pow(2, float64(attemptsMade-1)))
			return applyJitter(scaled, jitter), true
		}
	},
}

// Normalize converts a raw backoff input into BackoffOptions.
//
// Supported inputs:
// - int/int64: treated as milliseconds with type=fixed
// - time.Duration: treated as duration with type=fixed
// - BackoffOptions: returned as-is (with defaults)
func (Backoffs) Normalize(backoff any) (*BackoffOptions, error) {
	if backoff == nil {
		return nil, nil
	}

	switch v := backoff.(type) {
	case int:
		return &BackoffOptions{Type: "fixed", Delay: time.Duration(v) * time.Millisecond}, nil
	case int64:
		return &BackoffOptions{Type: "fixed", Delay: time.Duration(v) * time.Millisecond}, nil
	case time.Duration:
		return &BackoffOptions{Type: "fixed", Delay: v}, nil
	case BackoffOptions:
		out := v
		if out.Type == "" {
			out.Type = "fixed"
		}
		if out.Jitter < 0 {
			out.Jitter = 0
		}
		return &out, nil
	case *BackoffOptions:
		if v == nil {
			return nil, nil
		}
		out := *v
		if out.Type == "" {
			out.Type = "fixed"
		}
		if out.Jitter < 0 {
			out.Jitter = 0
		}
		return &out, nil
	default:
		return nil, errors.New("unsupported backoff type")
	}
}

// Calculate returns the delay for a retry attempt given normalized backoff options.
//
// attemptsMade is the number of attempts already made (1-based in BullMQ semantics).
func (Backoffs) Calculate(backoff BackoffOptions, attemptsMade int, err error, job MinimalJob, customStrategy func(attemptsMade int, err error, job MinimalJob) (time.Duration, bool)) (time.Duration, bool) {
	if customStrategy != nil {
		return customStrategy(attemptsMade, err, job)
	}

	strategyFactory := builtinStrategies[backoff.Type]
	if strategyFactory == nil {
		return 0, false
	}

	strategy := strategyFactory(backoff.Delay, backoff.Jitter)
	return strategy(attemptsMade, err, job)
}

func applyJitter(delay time.Duration, jitter float64) time.Duration {
	if delay <= 0 {
		return delay
	}
	if jitter <= 0 {
		return delay
	}
	if jitter > 1 {
		jitter = 1
	}

	min := float64(delay) * (1 - jitter)
	max := float64(delay) * (1 + jitter)
	if min < 0 {
		min = 0
	}

	//nolint:gosec // jitter is not used for security.
	v := min + rand.Float64()*(max-min)
	return time.Duration(v)
}
