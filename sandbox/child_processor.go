package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

// Receiver mirrors BullMQ's receiver concept (message channel from child).
// In this Go implementation, it is optional because responses are correlated by ID.
type Receiver interface {
	OnMessage(func(any))
}

// ChildProcessor acts as an interface between a child process and its parent.
// It processes one job at a time per child.
type ChildProcessor struct {
	send    func(job JobJsonSandbox, token string) (json.RawMessage, error)
	status  ChildStatus
	mu      sync.Mutex
	current Promise[json.RawMessage]
}

func NewChildProcessor(send func(job JobJsonSandbox, token string) (json.RawMessage, error)) *ChildProcessor {
	return &ChildProcessor{send: send, status: Idle}
}

func (p *ChildProcessor) Init(_ string) error {
	// Kept for API similarity with BullMQ.
	return nil
}

func (p *ChildProcessor) Start(ctx context.Context, jobJson JobJsonSandbox, token string) error {
	p.mu.Lock()
	if p.status == Terminating {
		p.mu.Unlock()
		return errors.New("child processor terminating")
	}
	p.status = Started
	promise := NewPromise(func() (json.RawMessage, error) {
		return p.send(jobJson, token)
	})
	p.current = promise
	p.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-promise:
		return res.Err
	}
}

func (p *ChildProcessor) Stop() error {
	p.mu.Lock()
	p.status = Terminating
	p.mu.Unlock()
	return nil
}

func (p *ChildProcessor) WaitForCurrentJobAndExit(ctx context.Context) error {
	p.mu.Lock()
	promise := p.current
	p.mu.Unlock()
	if promise == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-promise:
		return res.Err
	}
}
