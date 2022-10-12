package executor

import (
	"context"
	"io"
	"marketplace-yaga/linux/internal/executor/command"
	"marketplace-yaga/pkg/logger"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Executor struct {
	mu  sync.Mutex
	ctx context.Context

	timeout time.Duration
}

type ExecutorService interface {
	Run(command *command.Command) error
}

func (e *Executor) Run(command *command.Command) error {
	_, _, err := e.run(command)
	if err != nil {
		return err
	}

	return nil
}

func (e *Executor) RunO(command *command.Command) (string, error) {
	stdout, _, err := e.run(command)
	if err != nil {
		return "", err
	}

	return strings.Trim(stdout, "\n"), nil
}

func (e *Executor) run(command *command.Command) (string, string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	id := uuid.New().String()
	lgr := logger.FromContext(e.ctx).With(
		zap.String("id", id),
		zap.String("command", command.String()))
	lgr.Info("execute")

	var stdout, stderr strings.Builder
	err := runCommand(e.ctx, command, &stdout, &stderr, e.timeout)

	// if by any chance sensitive field could be cought,
	// in stdout/stderr, we must clear it out
	sensitiveArgumentsReplacer := command.SensitiveReplacer()
	clearedStdout := sensitiveArgumentsReplacer.Replace(stdout.String())
	clearedStderr := sensitiveArgumentsReplacer.Replace(stderr.String())

	lgr = lgr.With(
		zap.String("stdout", clearedStdout),
		zap.String("stderr", clearedStderr))
	if err != nil {
		lgr.Info("failed to execute command", zap.Error(err))

		return "", "", err
	}

	lgr.Info("command executed successfully")

	return clearedStdout, clearedStderr, nil
}

func runCommand(ctx context.Context, command *command.Command, stdout, stderr io.Writer, timeout time.Duration) error {
	ctxExec, cancel := context.WithTimeout(ctx, timeout) // context could be closed from above
	defer cancel()

	var arguments []string
	for _, a := range command.Arguments() {
		arguments = append(arguments, a.Value())
	}

	cmd := exec.CommandContext(ctxExec, arguments[0], arguments[1:]...)

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}
