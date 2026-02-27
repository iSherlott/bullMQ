package sandboxed

import (
	"context"

	bullmq "github.com/iSherlott/bullMQ"
)

// Consume delegates to the root consumer sandboxed mode.
//
// This package exists to keep a folder-based structure (`sandboxed/consumer.go`)
// similar to other features (e.g. `state/`, `redis/`, `sandbox/`).
func Consume(
	ctx context.Context,
	consumer *bullmq.Consumer,
	queueName string,
	process bullmq.SandboxedProcessOptions,
	options bullmq.ConsumerOptions,
) error {
	return consumer.ConsumeSandboxed(ctx, queueName, process, options)
}
