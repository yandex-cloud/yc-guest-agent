package argument

type ArgumentOption func(a *Argument)

//goland:noinspection GoUnusedExportedFunction
func Sensitive() ArgumentOption {
	return func(a *Argument) {
		a.sensitive = true
	}
}

//goland:noinspection GoUnusedExportedFunction
func NoEscape() ArgumentOption {
	return func(a *Argument) {
		a.noEscape = true
	}
}
