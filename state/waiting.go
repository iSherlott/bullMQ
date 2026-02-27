package state

const Waiting Status = "waiting"

const waitingSuffix = "wait"
const WaitingKeyType = "list"

func (k Keys) Waiting() string {
	return k.Prefix + ":" + k.Queue + ":" + waitingSuffix
}
