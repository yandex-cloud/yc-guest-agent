//go:build windows
// +build windows

package guest

import (
	"context"
	"errors"
	"fmt"
	"marketplace-yaga/pkg/heartbeat"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/pkg/meta"
	"marketplace-yaga/windows/internal/handlers/users"
	"marketplace-yaga/windows/internal/registry"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type Server struct {
	ctx       context.Context
	cancel    context.CancelFunc
	asService bool
	lastErr   error
}

var ErrUndefCtx = errors.New("expected context.Context")

var isWindowsService = svc.IsWindowsService

// NewServer creates instance of Server, enriches logger and detect if started as service.
func NewServer(ctx context.Context) (*Server, error) {
	if ctx == nil {
		return nil, ErrUndefCtx
	}

	s := Server{}

	var err error
	s.asService, err = isWindowsService()
	if err != nil {
		return nil, err
	}

	l := logger.FromContext(ctx).With(zap.String("server", "windows"))
	s.ctx, s.cancel = context.WithCancel(logger.NewContext(ctx, l))

	return &s, nil
}

const ServiceName = "yc-guest-agent"
const ServiceDescription = "Yandex.Cloud Guest Agent"

// getServiceManager is a global wrapped function for mocking in tests.
var getServiceManager = func() (svcMgr, error) { return mgr.Connect() }

type svcMgr interface {
	OpenService(name string) (*mgr.Service, error)
	CreateService(name string, exepath string, c mgr.Config, args ...string) (*mgr.Service, error)
	Disconnect() error
}

const (
	ServiceArgs     = "start"
	AgentDir        = `C:\Program Files\Yandex.Cloud\Guest Agent`
	AgentExecutable = "guest-agent.exe"
)

// Install creates windows service with for current executable.
func (s *Server) Install() (err error) {
	logger.InfoCtx(s.ctx, nil, "install executable as windows service")

	var p string
	p, err = os.Executable()
	if err != nil {
		logger.ErrorCtx(s.ctx, err, "get executable path")
		return
	}

	var m svcMgr
	m, err = getServiceManager()
	if err != nil {
		logger.ErrorCtx(s.ctx, err, "connect to service manager")
		return
	}
	defer func() {
		errDisc := m.Disconnect()
		if err == nil {
			err = errDisc
		}
	}()

	c := mgr.Config{DisplayName: ServiceName, StartType: mgr.StartAutomatic, Description: ServiceDescription}
	_, err = m.CreateService(ServiceName, p, c, ServiceArgs)
	if err != nil {
		logger.ErrorCtx(s.ctx, err, "create service", zap.String("config", fmt.Sprintf("%+v", c)))
	}

	return
}

// getServiceDeleter is a global wrapped function for mocking in tests.
var getServiceDeleter = func(s svcMgr) (svcDeleter, error) { return s.OpenService(ServiceName) }

type svcDeleter interface {
	Delete() error
}

// Uninstall removes windows service with well-known ServiceName.
func (s *Server) Uninstall() (err error) {
	logger.InfoCtx(s.ctx, nil, fmt.Sprintf("uninstall windows service: %v", ServiceName))

	var m svcMgr
	m, err = getServiceManager()
	if err != nil {
		logger.ErrorCtx(s.ctx, err, "connect to service manager")
		return
	}
	defer func() {
		errDisc := m.Disconnect()
		if err == nil {
			err = errDisc
		}
	}()

	var sc svcDeleter
	if sc, err = getServiceDeleter(m); err != nil {
		logger.ErrorCtx(s.ctx, err, "open service")
		return
	}

	err = sc.Delete()
	if err != nil {
		logger.ErrorCtx(s.ctx, err, "delete service")
	}

	return
}

const windowsKeyRegPath = `SOFTWARE\Yandex\Cloud\Compute`

// start initializes and starts agent.
func (s *Server) start() error {
	logger.InfoCtx(s.ctx, nil, "start agent")

	err := initRegistry(s.ctx)
	if err != nil {
		logger.ErrorCtx(s.ctx, err, "init registry",
			zap.String("registry HKLM relative path", windowsKeyRegPath))
		return err
	}

	err = startHeartbeat(s.ctx)
	if err != nil {
		logger.ErrorCtx(s.ctx, err, "start heartbeat")
		return err
	}

	logger.DebugCtx(s.ctx, nil, "start metadata watcher")
	startUserChangeMetadataWatcher(s.ctx)

	return nil
}

// createRegistryKey is a global wrapped function for mocking in tests.
var createRegistryKey = func() (bool, error) { return registry.CreateKey(windowsKeyRegPath) }

// initRegistry checks and creates registry key at windowsKeyRegPath.
func initRegistry(ctx context.Context) error {
	existed, err := createRegistryKey()
	if err != nil {
		logger.ErrorCtx(ctx, err, "check or create registry key")
		return err
	}
	if existed {
		logger.DebugCtx(ctx, nil, "key already existed")
	}

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

	// https://docs.microsoft.com/en-us/previous-versions/ms811896(v=msdn.10)#ucmgch09_topic3
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

	if s.asService {
		h := handler{s: s}
		logger.DebugCtx(s.ctx, nil, "start service stub", zap.String("config", fmt.Sprintf("%+v", h)))

		err = watchManagerEvents(ServiceName, &h)
		if err == nil {
			logger.ErrorCtx(s.ctx, err, "server exited")
			err = s.lastErr
			logger.ErrorCtx(s.ctx, err, "check additional errors")
		}
	} else {
		logger.DebugCtx(s.ctx, nil, "started from console")
		s.wait()
	}

	return nil
}

// subscribeOsSignals is a global wrapped function for mocking in tests.
var subscribeOsSignals = func(c chan<- os.Signal) { signal.Notify(c, syscall.SIGINT, syscall.SIGTERM) }

var watchManagerEvents = svc.Run

// handler used to handle events from Windows Service Manager, embed *Server to have access to context and logger.
type handler struct {
	s *Server
}

var ErrUnknownCmd = errors.New("service received unknown command")

const tickDuration = 50 * time.Millisecond

const commands = svc.AcceptStop | svc.AcceptShutdown

const interrogateStubDuration = 100 * time.Millisecond

// Execute is function that implement windows service control signals handling.
// https://docs.microsoft.com/en-us/windows/win32/api/winsvc/nc-winsvc-lphandler_function_ex
func (h *handler) Execute(_ []string, request <-chan svc.ChangeRequest, out chan<- svc.Status) (bool, uint32) {
	logger.InfoCtx(h.s.ctx, nil, "start service")

	out <- svc.Status{State: svc.StartPending}

	if h.s == nil {
		logger.DebugCtx(h.s.ctx, nil, "check if server field defined has failed")

		return true, 1
	}

	out <- svc.Status{State: svc.Running, Accepts: commands}
	ticker := time.NewTicker(tickDuration)

loop:
	for {
		select {
		case <-ticker.C:
		case <-h.s.ctx.Done(): // if someone send sigterm/kill signal to agent started as service
			logger.InfoCtx(h.s.ctx, nil, "received SIGTERM or SIGINT")

			break loop
		case c := <-request: // service part
			switch c.Cmd { //nolint:exhaustive
			case svc.Interrogate: // must report status
				logger.DebugCtx(h.s.ctx, nil, "received interrogate signal")
				out <- c.CurrentStatus
				// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(interrogateStubDuration)
				out <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				logger.InfoCtx(h.s.ctx, nil, "received STOP or SHUTDOWN signal")

				break loop
			default:
				h.s.lastErr = fmt.Errorf("%w: %v", ErrUnknownCmd, c.Cmd)
				logger.InfoCtx(h.s.ctx, h.s.lastErr, "received unsupported signal from windows service manager")

				return true, 2 //nolint:gomnd
			}
		}
	}
	logger.InfoCtx(h.s.ctx, nil, "stop service")
	out <- svc.Status{State: svc.StopPending}

	err := h.s.stop()
	logger.InfoCtx(h.s.ctx, err, "stop service")
	if err != nil {
		h.s.lastErr = err

		return true, 2 //nolint:gomnd
	}

	return false, 0
}
