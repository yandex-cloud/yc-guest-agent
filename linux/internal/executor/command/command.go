package command

import (
	"errors"
	"marketplace-yaga/linux/internal/executor/argument"
	"strings"
)

type Command struct {
	arguments []argument.Argument
}

func New(args ...argument.Argument) (*Command, error) {
	if len(args) == 0 {
		return nil, errors.New("no command provided")
	}

	return &Command{
		arguments: args,
	}, nil
}

func (c *Command) String() string {
	safeArguments := make([]string, len(c.arguments))

	for i, a := range c.arguments {
		safeArguments[i] = a.String()
	}

	return strings.Join(safeArguments, " ")
}

func (c *Command) Arguments() []argument.Argument {
	return c.arguments
}

// used to clean command output
func (c *Command) SensitiveReplacer() *strings.Replacer {
	var replacements []string

	for _, a := range c.arguments {
		if a.Sensitive() {
			replacements = append(replacements, a.Value(), a.String())
		}
	}

	return strings.NewReplacer(replacements...)
}
