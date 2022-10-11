package argument

import (
	"crypto/sha256"
	"fmt"

	"github.com/alessio/shellescape"
)

type Argument struct {
	argument string

	sensitive bool
	noEscape  bool
}

func New(argument string, options ...ArgumentOption) Argument {
	stripped := shellescape.StripUnsafe(argument)

	a := Argument{
		argument: stripped,
	}

	for _, option := range options {
		option(&a)
	}

	return a
}

func (a *Argument) Sensitive() bool {
	return a.sensitive
}

// String - implements Stringer interface, so Argument could be safely used outside
// without worries for sensitiveness. For example in fmt.Printf(...).
func (a *Argument) String() string {
	if a.sensitive {
		hash := sha256.Sum256([]byte(a.argument))
		return fmt.Sprintf("sensitive(sha256:%x)", hash)
	}

	return a.argument
}

func (a *Argument) Value() string {
	if a.noEscape {
		return a.argument
	}

	return shellescape.Quote(a.argument)
}
