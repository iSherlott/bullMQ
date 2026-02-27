package sandbox

import (
	"context"
	"sync"
)

type ChildPoolOpts struct {
	MainFile string
	SandboxedOptions
}

// ChildPool retains and reuses child processes keyed by processFile.
// This mirrors BullMQ's ChildPool semantics.
type ChildPool struct {
	mu sync.Mutex

	retained map[int]*Child
	free     map[string][]*Child

	opts ChildPoolOpts
}

func NewChildPool(opts ChildPoolOpts) *ChildPool {
	return &ChildPool{
		retained: make(map[int]*Child),
		free:     make(map[string][]*Child),
		opts:     opts,
	}
}

func (p *ChildPool) Retain(ctx context.Context, processFile string) (*Child, error) {
	p.mu.Lock()
	free := p.free[processFile]
	for len(free) > 0 {
		child := free[len(free)-1]
		free = free[:len(free)-1]
		if child != nil && !child.HasProcessExited() && child.Status() != Errored {
			p.free[processFile] = free
			p.retained[child.PID()] = child
			p.mu.Unlock()
			return child, nil
		}
	}
	p.free[processFile] = free
	p.mu.Unlock()

	child := NewChild(p.opts.MainFile, processFile, p.opts.SandboxedOptions)
	if err := child.Init(ctx); err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.retained[child.PID()] = child
	p.mu.Unlock()

	return child, nil
}

func (p *ChildPool) Release(child *Child) {
	if child == nil {
		return
	}
	if child.HasProcessExited() || child.Status() == Errored {
		p.Remove(child)
		return
	}

	p.mu.Lock()
	delete(p.retained, child.PID())
	p.free[child.processFile] = append(p.free[child.processFile], child)
	p.mu.Unlock()
}

func (p *ChildPool) Remove(child *Child) {
	if child == nil {
		return
	}

	p.mu.Lock()
	delete(p.retained, child.PID())
	free := p.free[child.processFile]
	out := free[:0]
	for _, c := range free {
		if c != child {
			out = append(out, c)
		}
	}
	p.free[child.processFile] = out
	p.mu.Unlock()
}

func (p *ChildPool) Kill(ctx context.Context, child *Child) error {
	if child == nil {
		return nil
	}
	p.Remove(child)
	return child.Stop(ctx)
}

func (p *ChildPool) Clean(ctx context.Context) error {
	p.mu.Lock()
	children := make([]*Child, 0, len(p.retained))
	for _, c := range p.retained {
		children = append(children, c)
	}
	for _, list := range p.free {
		children = append(children, list...)
	}
	p.retained = make(map[int]*Child)
	p.free = make(map[string][]*Child)
	p.mu.Unlock()

	var firstErr error
	for _, c := range children {
		if err := c.Stop(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (p *ChildPool) GetFree(processFile string) []*Child {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]*Child, 0, len(p.free[processFile]))
	out = append(out, p.free[processFile]...)
	return out
}

func (p *ChildPool) GetAllFree() []*Child {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []*Child
	for _, list := range p.free {
		out = append(out, list...)
	}
	return out
}
