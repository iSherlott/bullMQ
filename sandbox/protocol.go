package sandbox

import (
	"encoding/json"
	"time"
)

type JobJsonSandbox struct {
	ID        string          `json:"id"`
	QueueName string          `json:"queueName"`
	Name      string          `json:"name"`
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"`
}

type requestType string

type request struct {
	ID    uint64          `json:"id"`
	Type  requestType     `json:"type"`
	Job   *JobJsonSandbox `json:"job,omitempty"`
	Token string          `json:"token,omitempty"`
}

type responseType string

type response struct {
	ID     uint64          `json:"id"`
	Type   responseType    `json:"type"`
	Error  string          `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	TookMs int64           `json:"tookMs,omitempty"`
}

const (
	requestInit  requestType = "init"
	requestStart requestType = "start"
	requestStop  requestType = "stop"
)

const (
	responseReady   responseType = "ready"
	responseResult  responseType = "result"
	responseStopped responseType = "stopped"
	responseError   responseType = "error"
)

func nowMillis() int64 {
	return time.Now().UTC().UnixMilli()
}
