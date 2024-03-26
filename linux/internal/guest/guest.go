package guest

import (
	"context"
	"errors"
	"marketplace-yaga/linux/internal/handlers/kmssecrets"
	"marketplace-yaga/linux/internal/handlers/lockboxsecrets"
	"marketplace-yaga/linux/internal/handlers/managedcertificates"
	"marketplace-yaga/linux/internal/handlers/sshkeys"
	"marketplace-yaga/linux/internal/handlers/users"
	"marketplace-yaga/pkg/heartbeat"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/pkg/meta"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

type Server struct {
	ctx       context.Context
	cancel    context.CancelFunc
	asService bool
	lastErr   error
}

var ErrUndefCtx = errors.New("expected context.Context")

// NewServer creates instance of Server, enriches logger.
func NewServer(ctx context.Context) (*Server, error) {
	if ctx == nil {
		return nil, ErrUndefCtx
	}

	s := Server{}

	l := logger.FromContext(ctx).With(zap.String("server", "linux"))
	s.ctx, s.cancel = context.WithCancel(logger.NewContext(ctx, l))

	return &s, nil
}

// start initializes and starts agent.
func (s *Server) start() error {
	logger.InfoCtx(s.ctx, nil, "start agent")

	err := startHeartbeat(s.ctx)
	if err != nil {
		logger.ErrorCtx(s.ctx, err, "start heartbeat")
		return err
	}

	logger.DebugCtx(s.ctx, nil, "start metadata watcher")
	startUserChangeMetadataWatcher(s.ctx)

	return nil
}

type starter interface {
	Start() error
}

// createHeartbeatSerialTicker is a global wrapped function for mocking in tests.
var createHeartbeatSerialTicker = func(ctx context.Context) (starter, error) {
	return heartbeat.NewSerialTicker(ctx)
}

// startHeartbeat starts to send heartbeat messages to serial port.
func startHeartbeat(ctx context.Context) error {
	hb, err := createHeartbeatSerialTicker(ctx)
	if err != nil {
		logger.ErrorCtx(ctx, err, "create heartbeat ticker")
		return err
	}

	err = hb.Start()
	if err != nil {
		logger.ErrorCtx(ctx, err, "start heartbeat")
	}
	return err
}

// startUserChangeMetadataWatcher starts poller for user change request messages.
func startUserChangeMetadataWatcher(ctx context.Context) {
	logger.DebugCtx(ctx, nil, "create metadata watcher")
	w := meta.NewMetadataWatcher(ctx)

	logger.DebugCtx(ctx, nil, "add metadata watcher")
	w.AddWatch(sshkeys.DefaultMetadataURL, sshkeys.NewUserHandler())
	w.AddWatch(kmssecrets.DefaultMetadataURL, kmssecrets.NewKmsHandler())
	w.AddWatch(lockboxsecrets.DefaultMetadataURL, lockboxsecrets.NewLockboxHandler())
	w.AddWatch(managedcertificates.DefaultMetadataURL, managedcertificates.CertificatesHandler())
	w.AddWatch(users.DefaultMetadataURL, users.NewUserHandle())
}

var ErrStopTimeout = errors.New("timeout stopping service")

const stopTimeout = 10 * time.Second

// stop closes context and waits stopTimeout.
func (s *Server) stop() (err error) {
	logger.DebugCtx(s.ctx, nil, "cancel context")
	s.cancel()

	select {
	case <-s.ctx.Done():
		logger.DebugCtx(s.ctx, nil, "context closed")
	case <-time.After(stopTimeout):
		err = ErrStopTimeout
		logger.ErrorCtx(s.ctx, err, "gave up waiting for context close")
	}

	return
}

func (s *Server) wait() {
	logger.DebugCtx(s.ctx, nil, "wait context to close")
	<-s.ctx.Done()
}

// Run start agent and handles OS or Service Manger's signals/events.
func (s *Server) Run() error {
	err := s.start()
	if err != nil {
		logger.ErrorCtx(s.ctx, err, "start server")
		return err
	}

	// gracefully react on sigTerm/break
	c := make(chan os.Signal, 1)
	subscribeOsSignals(c)

	var sigErr error
	go func() {
		<-c
		logger.InfoCtx(s.ctx, nil, "received SIGTERM or SIGINT")
		sigErr = s.stop()
	}()
	defer func() {
		if err == nil {
			err = sigErr
		}
	}()

	logger.DebugCtx(s.ctx, nil, "started from console")
	s.wait()

	return nil
}

// subscribeOsSignals is a global wrapped function for mocking in tests.
var subscribeOsSignals = func(c chan<- os.Signal) { signal.Notify(c, syscall.SIGINT, syscall.SIGTERM) }
