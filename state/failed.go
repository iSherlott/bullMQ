package state

const Failed Status = "failed"

const failedSuffix = "failed"
const FailedKeyType = "zset"

func (k Keys) Failed() string {
	return k.Prefix + ":" + k.Queue + ":" + failedSuffix
}
