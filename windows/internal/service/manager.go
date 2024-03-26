package service

import (
	"context"
	"errors"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/windows/internal/guest"
	mocks2 "marketplace-yaga/windows/internal/service/mocks"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func NewManager(ctx context.Context) (*Manager, error) {
	if ctx == nil {
		return nil, errors.New("provided nil context")
	}

	return &Manager{ctx: ctx}, nil
}

type Manager struct {
	ctx           context.Context
	mgr           manager
	openService   serviceOpener
	createService serviceCreator
}

//go:generate mockery --name manager --exported --disable-version-string --tags windows

var _ manager = &mocks2.Manager{}

type manager interface {
	CreateService(name string, exepath string, c mgr.Config, args ...string) (*mgr.Service, error)
	OpenService(name string) (*mgr.Service, error)
	ListServices() ([]string, error)
	Disconnect() error
}

type serviceOpener func(name string) (service, error)

func newServiceOpener(m manager) serviceOpener {
	return func(name string) (service, error) {
		return m.OpenService(name)
	}
}

//go:generate mockery --name service --exported --disable-version-string --tags windows

var _ service = &mocks2.Service{}

type service interface {
	Start(args ...string) error
	Control(c svc.Cmd) (svc.Status, error)
	Query() (svc.Status, error)
	Delete() error
	Close() error
}

type serviceCreator func(name, displayName, description, path string) error

func newServiceCreator(m manager) serviceCreator {
	return func(name, displayName, description, path string) error {
		c := mgr.Config{DisplayName: displayName, StartType: mgr.StartAutomatic, Description: description}
		_, err := m.CreateService(name, path, c)

		return err
	}
}

func (m *Manager) Init() error {
	mg, err := mgr.Connect()
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "connect service manager")
		return err
	}

	m.openService = newServiceOpener(mg)
	m.createService = newServiceCreator(mg)
	m.mgr = mg

	return err
}

func (m *Manager) Close() error {
	err := m.mgr.Disconnect()
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "close service manager")
	}
	m.mgr = nil

	return err
}

func (m *Manager) IsExist(name string) (bool, error) {
	services, err := m.mgr.ListServices()
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "list services",
			zap.Strings("services", services))
		return false, err
	}

	logger.DebugCtx(m.ctx, nil, "looking for service",
		zap.String("name", name))
	for _, s := range services {
		if s == name {
			return true, nil
		}
	}

	return false, nil
}

func (m *Manager) IsStopped(name string) (bool, error) {
	s, err := m.getStatus(name)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "get status",
			zap.String("name", name),
			zap.String("status", s.String()))
		return false, err
	}

	return s == Stopped, nil
}

func (m *Manager) IsRunning(name string) (bool, error) {
	s, err := m.getStatus(name)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "get status",
			zap.String("name", name),
			zap.String("status", s.String()))
		return false, err
	}

	return s == Running, nil
}

func (m *Manager) getStatus(name string) (State, error) {
	e, err := m.IsExist(name)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "check exist",
			zap.String("name", name),
			zap.Bool("exist", e))
		return Unknown, err
	}
	if !e {
		return Unknown, ErrNotFound
	}

	s, err := m.openService(name)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "open service",
			zap.String("name", name))
		return Unknown, err
	}
	defer func() {
		_ = s.Close()
	}()

	r, err := s.Query()
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "query service",
			zap.String("name", name),
			zap.Stringer("state", State(r.State)))
		return Unknown, err
	}

	return State(r.State), nil
}

const Timeout = 60 * time.Second

var timeout = Timeout

func (m *Manager) Start(name string) error {
	e, err := m.IsExist(name)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "check exist",
			zap.String("name", name),
			zap.Bool("exist", e))
		return err
	}
	if !e {
		return ErrNotFound
	}

	running, err := m.IsRunning(name)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "check running",
			zap.String("name", name),
			zap.Bool("running", running))
		return err
	}
	if running {
		return nil
	}

	s, err := m.openService(name)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "open service",
			zap.String("name", name))
		return err
	}
	defer func() {
		_ = s.Close()
	}()

	err = s.Start()
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "start service",
			zap.String("name", name))
		return err
	}

	t := time.NewTicker(1 * time.Second)
	select {
	case <-t.C:
		running, err = m.IsRunning(name)
		if err != nil {
			logger.ErrorCtx(m.ctx, err, "check running",
				zap.String("name", name),
				zap.Bool("running", running))
			return err
		}
		if running {
			return nil
		}
	case <-time.After(timeout):
		break
	}
	t.Stop()

	return ErrTimeout
}

func (m *Manager) Stop(name string) error {
	e, err := m.IsExist(name)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "check exist",
			zap.String("name", name),
			zap.Bool("exist", e))
		return err
	}
	if !e {
		return ErrNotFound
	}

	stopped, err := m.IsStopped(name)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "check stopped",
			zap.String("name", name),
			zap.Bool("stopped", stopped))
		return err
	}
	if stopped {
		return nil
	}

	s, err := m.openService(guest.ServiceName)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "open service",
			zap.String("name", name))
		return err
	}
	defer func() {
		_ = s.Close()
	}()

	_, err = s.Control(svc.Stop)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "stop service",
			zap.String("name", name))
		return err
	}

	t := time.NewTicker(1 * time.Second)
	select {
	case <-t.C:
		stopped, err = m.IsStopped(name)
		if err != nil {
			logger.ErrorCtx(m.ctx, err, "check stopped",
				zap.String("name", name),
				zap.Bool("stopped", stopped))
			return err
		}
		if stopped {
			return nil
		}
	case <-time.After(timeout):
		break
	}
	t.Stop()

	return ErrTimeout
}

func (m *Manager) Create(path, name, displayName, description string, args ...string) error {
	e, err := m.IsExist(name)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "check exist",
			zap.String("name", name),
			zap.Bool("exist", e))
		return err
	}
	if e {
		return ErrAlreadyExist
	}

	c := mgr.Config{DisplayName: displayName, StartType: mgr.StartAutomatic, Description: description}
	_, err = m.mgr.CreateService(name, path, c, args...)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "create service",
			zap.String("path", path),
			zap.String("name", name),
			zap.String("displayName", displayName),
			zap.String("description", description),
			zap.Strings("args", args))
		return err
	}

	return nil
}

func (m *Manager) Delete(name string) (err error) {
	e, err := m.IsExist(name)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "check exist",
			zap.String("name", name),
			zap.Bool("exist", e))
		return
	}
	if !e {
		return ErrNotFound
	}

	s, err := m.openService(name)
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "open service",
			zap.String("name", name))
		return err
	}
	defer func() {
		_ = s.Close()
	}()

	err = s.Delete()
	if err != nil {
		logger.ErrorCtx(m.ctx, err, "delete service",
			zap.String("name", name))
	}

	return
}
