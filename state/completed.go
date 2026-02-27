package state

const Completed Status = "completed"

const completedSuffix = "completed"
const CompletedKeyType = "zset"

func (k Keys) Completed() string {
	return k.Prefix + ":" + k.Queue + ":" + completedSuffix
}
