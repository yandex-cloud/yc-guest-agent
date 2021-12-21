package service

type State int

const (
	Unknown State = iota
	Stopped
	StartPending
	StopPending
	Running
	ContinuePending
	PausePending
	Paused
)

func (e State) String() string {
	return [...]string{
		"Unknown",
		"Stopped",
		"StartPending",
		"StopPending",
		"Running",
		"ContinuePending",
		"PausePending",
		"Paused"}[e]
}
