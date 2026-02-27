package bullmq

import (
	"context"
	"sync"
)

// Result is the resolved value of a Promise.
type Result[T any] struct {
	Value T
	Err   error
}

// Promise is a minimal Go equivalent of a JS Promise that resolves once.
// It is represented as a receive-only channel carrying a single Result.
type Promise[T any] <-chan Result[T]

// NewPromise runs fn in a goroutine and returns a Promise that resolves once.
func NewPromise[T any](fn func() (T, error)) Promise[T] {
	ch := make(chan Result[T], 1)
	go func() {
		var zero T
		v, err := fn()
		if err != nil {
			ch <- Result[T]{Value: zero, Err: err}
			return
		}
		ch <- Result[T]{Value: v, Err: nil}
	}()
	return ch
}

// AsyncFifoQueue is a minimal FIFO queue for asynchronous operations.
// Promises are queued in the order they are resolved (not added).
//
// This mirrors BullMQ's AsyncFifoQueue semantics but uses Go channels.
type AsyncFifoQueue[T any] struct {
	ignoreErrors bool

	mu      sync.Mutex
	queue   []Result[T]
	pending int
	total   int

	changed chan struct{}
}

func NewAsyncFifoQueue[T any](ignoreErrors bool) *AsyncFifoQueue[T] {
	return &AsyncFifoQueue[T]{
		ignoreErrors: ignoreErrors,
		changed:      make(chan struct{}),
	}
}

// Add registers a promise for tracking. When it resolves, it is enqueued.
func (q *AsyncFifoQueue[T]) Add(promise Promise[T]) {
	if promise == nil {
		return
	}

	q.mu.Lock()
	q.pending++
	q.total++
	q.signalLocked()
	q.mu.Unlock()

	go func() {
		result, ok := <-promise
		if !ok {
			q.mu.Lock()
			q.pending--
			q.signalLocked()
			q.mu.Unlock()
			return
		}

		q.mu.Lock()
		q.pending--
		if !(q.ignoreErrors && result.Err != nil) {
			q.queue = append(q.queue, result)
		}
		q.signalLocked()
		q.mu.Unlock()
	}()
}

// WaitAll blocks until all pending promises have resolved.
func (q *AsyncFifoQueue[T]) WaitAll(ctx context.Context) error {
	for {
		q.mu.Lock()
		if q.pending == 0 {
			q.mu.Unlock()
			return nil
		}
		ch := q.changed
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
	}
}

func (q *AsyncFifoQueue[T]) NumTotal() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.total
}

func (q *AsyncFifoQueue[T]) NumPending() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pending
}

func (q *AsyncFifoQueue[T]) NumQueued() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}

// Fetch returns the next resolved result in FIFO order.
//
// If there are no queued results and no pending promises, it returns (zero, false, nil).
// If ctx is canceled while waiting, it returns (zero, false, ctx.Err()).
func (q *AsyncFifoQueue[T]) Fetch(ctx context.Context) (T, bool, error) {
	var zero T

	for {
		q.mu.Lock()
		if len(q.queue) > 0 {
			result := q.queue[0]
			q.queue = q.queue[1:]
			q.signalLocked()
			q.mu.Unlock()
			if result.Err != nil {
				return zero, true, result.Err
			}
			return result.Value, true, nil
		}
		if q.pending == 0 {
			q.mu.Unlock()
			return zero, false, nil
		}
		ch := q.changed
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return zero, false, ctx.Err()
		case <-ch:
		}
	}
}

func (q *AsyncFifoQueue[T]) signalLocked() {
	// Broadcast to waiters without a condition variable, but still allowing context cancellation.
	close(q.changed)
	q.changed = make(chan struct{})
}
