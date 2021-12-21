package updater

type State int

const (
	Noop State = iota
	Download
	Update
	DownloadAndUpdate
	Install
	DownloadAndInstall
	Unknown
)

func (e State) String() string {
	return [...]string{
		"Noop",               // 0
		"Download",           // 1
		"Update",             // 2
		"DownloadAndUpdate",  // 3 = 1 + 2 -> Download + Update
		"Install",            // 4
		"DownloadAndInstall", // 5 = 1 + 4 -> Download + Install
		"Unknown"}[e]
}
