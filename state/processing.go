package state

const Processing Status = "processing"

const processingSuffix = "active"
const ProcessingKeyType = "list"

func (k Keys) Processing() string {
	return k.Prefix + ":" + k.Queue + ":" + processingSuffix
}
