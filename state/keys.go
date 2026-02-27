package state

type Status string

type Keys struct {
	Prefix string
	Queue  string
}

func NewKeys(prefix string, queue string) Keys {
	return Keys{Prefix: prefix, Queue: queue}
}

func (k Keys) JobID() string {
	return k.Prefix + ":" + k.Queue + ":id"
}

func (k Keys) Job(jobID string) string {
	return k.Prefix + ":" + k.Queue + ":" + jobID
}
