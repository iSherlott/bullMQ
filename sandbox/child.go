package sandbox

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

type ChildStatus int

const (
	Idle ChildStatus = iota
	Started
	Terminating
	Errored
)

// Child is a process wrapper used to execute sandboxed job processing.
// It communicates using JSON objects over stdin/stdout.
type Child struct {
	processFile string
	mainFile    string
	opts        SandboxedOptions

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	encMu sync.Mutex
	enc   *json.Encoder

	statusMu sync.Mutex
	status   ChildStatus

	requestID atomic.Uint64

	pendingMu sync.Mutex
	pending   map[uint64]chan response

	readDone chan struct{}
}

type SandboxedOptions struct {
	Dir         string
	Env         []string
	KillTimeout time.Duration
}

func NewChild(mainFile string, processFile string, opts SandboxedOptions) *Child {
	return &Child{
		processFile: processFile,
		mainFile:    mainFile,
		opts:        opts,
		pending:     make(map[uint64]chan response),
		readDone:    make(chan struct{}),
	}
}

func (c *Child) PID() int {
	if c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}

func (c *Child) Status() ChildStatus {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	return c.status
}

func (c *Child) Init(ctx context.Context) error {
	c.statusMu.Lock()
	if c.status != Idle {
		c.statusMu.Unlock()
		return nil
	}
	c.status = Started
	c.statusMu.Unlock()

	var cmd *exec.Cmd
	if c.mainFile != "" {
		cmd = exec.CommandContext(ctx, c.mainFile, c.processFile)
	} else {
		cmd = exec.CommandContext(ctx, c.processFile)
	}

	if c.opts.Dir != "" {
		cmd.Dir = c.opts.Dir
	}
	if len(c.opts.Env) > 0 {
		cmd.Env = append(os.Environ(), c.opts.Env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		c.setErroredLocked(err)
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.setErroredLocked(err)
		_ = stdin.Close()
		return err
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		c.setErroredLocked(err)
		_ = stdin.Close()
		_ = stdout.Close()
		return err
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = stdout
	c.enc = json.NewEncoder(stdin)

	go c.readLoop()

	// optional init handshake (non-fatal if child doesn't respond)
	_ = c.sendNoWait(request{ID: c.nextID(), Type: requestInit})

	return nil
}

func (c *Child) HasProcessExited() bool {
	if c.cmd == nil {
		return true
	}
	if c.cmd.ProcessState == nil {
		return false
	}
	return c.cmd.ProcessState.Exited()
}

func (c *Child) Send(ctx context.Context, job JobJsonSandbox, token string) (json.RawMessage, error) {
	if c.cmd == nil {
		return nil, errors.New("child not initialized")
	}

	reqID := c.nextID()
	ch := make(chan response, 1)

	c.pendingMu.Lock()
	c.pending[reqID] = ch
	c.pendingMu.Unlock()

	err := c.sendNoWait(request{ID: reqID, Type: requestStart, Job: &job, Token: token})
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, reqID)
		c.pendingMu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, reqID)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != "" {
			return nil, errors.New(resp.Error)
		}
		return resp.Result, nil
	}
}

func (c *Child) Stop(ctx context.Context) error {
	c.statusMu.Lock()
	if c.status == Terminating {
		c.statusMu.Unlock()
		return nil
	}
	c.status = Terminating
	c.statusMu.Unlock()

	if c.cmd == nil {
		return nil
	}

	_ = c.sendNoWait(request{ID: c.nextID(), Type: requestStop})

	done := make(chan error, 1)
	go func() { done <- c.cmd.Wait() }()

	timeout := c.opts.KillTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	select {
	case <-ctx.Done():
		_ = c.Kill("SIGKILL")
		return ctx.Err()
	case err := <-done:
		return err
	case <-time.After(timeout):
		_ = c.Kill("SIGKILL")
		return errors.New("child stop timeout; killed")
	}
}

func (c *Child) Kill(signal string) error {
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	// Windows does not support signals like SIGKILL reliably; Process.Kill is best effort.
	return c.cmd.Process.Kill()
}

func (c *Child) readLoop() {
	defer close(c.readDone)

	reader := bufio.NewReader(c.stdout)
	dec := json.NewDecoder(reader)

	for {
		var resp response
		if err := dec.Decode(&resp); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			c.setErroredLocked(err)
			c.failAllPending(err)
			return
		}

		c.pendingMu.Lock()
		ch := c.pending[resp.ID]
		if ch != nil {
			delete(c.pending, resp.ID)
		}
		c.pendingMu.Unlock()

		if ch != nil {
			ch <- resp
		}
	}
}

func (c *Child) failAllPending(err error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()

	for id, ch := range c.pending {
		delete(c.pending, id)
		ch <- response{ID: id, Type: responseError, Error: err.Error()}
	}
}

func (c *Child) nextID() uint64 {
	return c.requestID.Add(1)
}

func (c *Child) sendNoWait(req request) error {
	c.encMu.Lock()
	defer c.encMu.Unlock()

	if c.enc == nil {
		return errors.New("child encoder not ready")
	}

	if err := c.enc.Encode(req); err != nil {
		c.setErroredLocked(err)
		return fmt.Errorf("send to child: %w", err)
	}

	return nil
}

func (c *Child) setErroredLocked(err error) {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	c.status = Errored
}
