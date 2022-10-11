package executor

import (
	"context"
	"time"
)

type Builder struct {
	exec *Executor
}

func NewBuilder(ctx context.Context) *Builder {
	return &Builder{
		exec: &Executor{
			ctx:     ctx,
			timeout: 60 * time.Second,
		},
	}
}

func (b *Builder) WithTimeout(t time.Duration) *Builder {
	b.exec.timeout = t

	return b
}

func (b *Builder) Build() *Executor {
	if b.exec == nil {
		return &Executor{}
	}

	return b.exec
}
