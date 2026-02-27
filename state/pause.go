package state

const Paused Status = "paused"

const pausedSuffix = "paused"
const PausedKeyType = "list"

func (k Keys) Paused() string {
	return k.Prefix + ":" + k.Queue + ":" + pausedSuffix
}
