package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

// ProcessorFunc is the function executed in the child process for each job.
// It should write logs to stderr (not stdout), since stdout is reserved for protocol messages.
type ProcessorFunc func(ctx context.Context, job JobJsonSandbox, token string) (any, error)

// RunChild runs a protocol loop reading requests from stdin and writing responses to stdout.
//
// Build a child binary that calls RunChild in its main(), and then run it via ChildPool/Child.
func RunChild(ctx context.Context, processor ProcessorFunc) error {
	if processor == nil {
		return errors.New("processor is required")
	}

	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	for {
		var req request
		if err := dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		switch req.Type {
		case requestInit:
			_ = enc.Encode(response{ID: req.ID, Type: responseReady})
		case requestStop:
			_ = enc.Encode(response{ID: req.ID, Type: responseStopped})
			return nil
		case requestStart:
			start := time.Now().UTC()
			var job JobJsonSandbox
			if req.Job != nil {
				job = *req.Job
			}

			result, err := processor(ctx, job, req.Token)
			var raw json.RawMessage
			if err == nil && result != nil {
				if b, marshalErr := json.Marshal(result); marshalErr == nil {
					raw = b
				}
			}

			resp := response{ID: req.ID, Type: responseResult, Result: raw, TookMs: time.Since(start).Milliseconds()}
			if err != nil {
				resp.Error = err.Error()
			}
			_ = enc.Encode(resp)
		default:
			_ = enc.Encode(response{ID: req.ID, Type: responseError, Error: "unknown request type"})
		}
	}
}
