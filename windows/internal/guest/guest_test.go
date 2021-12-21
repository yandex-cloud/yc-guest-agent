package guest

import (
	"context"
	"fmt"
	"marketplace-yaga/pkg/logger"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type isWindowsServiceMock struct{ mock.Mock }

func (m *isWindowsServiceMock) isWindowsService() (bool, error) {
	args := m.Called()

	return args.Bool(0), args.Error(1)
}

func TestNewServer(t *testing.T) {
	suite.Run(t, new(newServerTests))
}

type newServerTests struct{ suite.Suite }

func (s *newServerTests) TestNewServerService() {
	lc := logger.NewContext(context.Background(), zaptest.NewLogger(s.T()))
	ctx, cancel := context.WithCancel(lc)
	defer cancel()
	tests := []struct {
		ctx     context.Context
		retRes  bool
		wantErr bool
		retErr  error
	}{
		{ctx: ctx, retRes: false, retErr: assert.AnError, wantErr: true},
		{ctx: ctx, retRes: false, retErr: nil, wantErr: false},
		{ctx: ctx, retRes: true, retErr: assert.AnError, wantErr: true},
		{ctx: ctx, retRes: true, retErr: nil, wantErr: false},
		{ctx: nil, retRes: false, retErr: assert.AnError, wantErr: true},
		{ctx: nil, retRes: false, retErr: nil, wantErr: true},
		{ctx: nil, retRes: true, retErr: assert.AnError, wantErr: true},
		{ctx: nil, retRes: true, retErr: nil, wantErr: true},
	}

	for _, t := range tests {
		i := new(isWindowsServiceMock)
		i.On("isWindowsService").Return(t.retRes, t.retErr)
		isWindowsService = i.isWindowsService
		srv, err := NewServer(t.ctx)

		if t.wantErr {
			s.Error(err)
			s.Nil(srv)
		} else {
			s.NoError(err)
			s.IsType(&Server{}, srv)
			s.IsType(t.ctx, srv.ctx)
			s.IsType(cancel, srv.cancel)
			s.Equal(t.retRes, srv.asService)
		}
	}
}

type srvMgrMock struct{ mock.Mock }

func (m *srvMgrMock) Disconnect() error {
	args := m.Called()

	return args.Error(0)
}

func (m *srvMgrMock) CreateService(name string, exepath string, c mgr.Config, _ ...string) (*mgr.Service, error) {
	args := m.Called(name, exepath, c)

	return args.Get(0).(*mgr.Service), args.Error(1)
}

func (m *srvMgrMock) OpenService(name string) (*mgr.Service, error) {
	args := m.Called(name)

	return args.Get(0).(*mgr.Service), args.Error(1)
}

func TestInstall(t *testing.T) {
	suite.Run(t, new(installTests))
}

type installTests struct{ suite.Suite }

func (s *installTests) TestInstall() {
	cfg := mgr.Config{DisplayName: ServiceName, StartType: mgr.StartAutomatic, Description: ServiceDescription}

	tests := []struct {
		retCrSvc    *mgr.Service
		retCrSvcErr error
		retDiscErr  error
		wantErr     bool
	}{
		{retCrSvc: nil, retCrSvcErr: assert.AnError, retDiscErr: nil, wantErr: true},
		{retCrSvc: &mgr.Service{}, retCrSvcErr: nil, retDiscErr: nil, wantErr: false},
		{retCrSvc: nil, retCrSvcErr: assert.AnError, retDiscErr: assert.AnError, wantErr: true},
		{retCrSvc: &mgr.Service{}, retCrSvcErr: nil, retDiscErr: assert.AnError, wantErr: true},
	}

	for _, t := range tests {
		m := new(srvMgrMock)
		m.On("Disconnect").Return(t.retDiscErr)
		m.On("CreateService", ServiceName, mock.Anything, cfg).Return(t.retCrSvc, t.retCrSvcErr)
		getServiceManager = func() (svcMgr, error) { return m, nil }

		lc := logger.NewContext(context.Background(), zaptest.NewLogger(s.T()))
		srv, err := NewServer(lc)
		s.NoError(err)

		if t.wantErr {
			s.Error(srv.Install(), fmt.Sprintf("%+v", t))
		} else {
			s.NoError(srv.Install(), fmt.Sprintf("%+v", t))
		}

		m.AssertCalled(s.T(), "CreateService", ServiceName, mock.Anything, cfg)
		m.AssertCalled(s.T(), "Disconnect")
	}
}

type svcDeleterMock struct{ mock.Mock }

func (m *svcDeleterMock) Delete() error {
	args := m.Called()

	return args.Error(0)
}

func TestUninstall(t *testing.T) {
	suite.Run(t, new(uninstallTests))
}

type uninstallTests struct{ suite.Suite }

func (s *uninstallTests) TestUninstall() {
	tests := []struct {
		retSvcErr  error
		retDiscErr error
		retDltrErr error
		wantErr    bool
	}{
		{retSvcErr: nil, retDiscErr: nil, retDltrErr: nil, wantErr: false},
		{retSvcErr: nil, retDiscErr: nil, retDltrErr: assert.AnError, wantErr: true},
		{retSvcErr: nil, retDiscErr: assert.AnError, retDltrErr: assert.AnError, wantErr: true},
		{retSvcErr: assert.AnError, retDiscErr: assert.AnError, retDltrErr: assert.AnError, wantErr: true},
		{retSvcErr: assert.AnError, retDiscErr: assert.AnError, retDltrErr: nil, wantErr: true},
		{retSvcErr: assert.AnError, retDiscErr: nil, retDltrErr: nil, wantErr: true},
		{retSvcErr: nil, retDiscErr: assert.AnError, retDltrErr: nil, wantErr: true},
	}

	for _, t := range tests {
		d := new(svcDeleterMock)
		d.On("Delete").Return(t.retDltrErr)
		getServiceDeleter = func(s svcMgr) (svcDeleter, error) { return d, t.retSvcErr }

		m := new(srvMgrMock)
		m.On("Disconnect").Return(t.retDiscErr)
		m.On("OpenService", ServiceName).Return(mock.Anything, t.retSvcErr)
		getServiceManager = func() (svcMgr, error) { return m, nil }

		lc := logger.NewContext(context.Background(), zaptest.NewLogger(s.T()))
		srv, err := NewServer(lc)
		s.NoError(err)

		if t.wantErr {
			s.Error(srv.Uninstall(), fmt.Sprintf("%+v", t))
		} else {
			s.NoError(srv.Uninstall(), fmt.Sprintf("%+v", t))
		}
	}
}

type heartbeatSerialTickerMock struct{ mock.Mock }

func (m *heartbeatSerialTickerMock) Start() error {
	args := m.Called()

	return args.Error(0)
}

func TestStart(t *testing.T) {
	suite.Run(t, new(startTests))
}

type startTests struct{ suite.Suite }

func (s *startTests) TestStart() {
	tests := []struct {
		retCreateRegistryKeyExist bool
		retCreateRegistryKeyErr   error
		retCreateSerialTickerErr  error
		retStartSerialTickerErr   error
		wantErr                   bool
	}{
		{true, nil, nil, nil, false},
		{true, assert.AnError, assert.AnError, assert.AnError, true},
		{true, assert.AnError, assert.AnError, nil, true},
		{true, assert.AnError, nil, nil, true},
		{true, nil, nil, assert.AnError, true},
		{true, nil, assert.AnError, assert.AnError, true},
		{true, nil, assert.AnError, nil, true},
		{false, assert.AnError, assert.AnError, assert.AnError, true},
		{false, assert.AnError, assert.AnError, nil, true},
		{false, assert.AnError, nil, nil, true},
		{false, nil, nil, assert.AnError, true},
		{false, nil, assert.AnError, assert.AnError, true},
		{false, nil, assert.AnError, nil, true},
		{false, nil, nil, nil, false},
	}

	for _, t := range tests {
		createRegistryKey = func() (bool, error) {
			return t.retCreateRegistryKeyExist, t.retCreateRegistryKeyErr
		}

		h := new(heartbeatSerialTickerMock)
		h.On("Start").Return(t.retStartSerialTickerErr)
		createHeartbeatSerialTicker = func(ctx context.Context) (starter, error) {
			return h, t.retCreateSerialTickerErr
		}

		ctx := logger.NewContext(context.Background(), zaptest.NewLogger(s.T()))
		srv, err := NewServer(ctx)
		s.NoError(err)

		if t.wantErr {
			s.Error(srv.start(), fmt.Sprintf("%+v\n", t))
		} else {
			s.NoError(srv.start(), fmt.Sprintf("%+v\n", t))
		}
	}
}

func TestStop(t *testing.T) {
	suite.Run(t, new(stopTests))
}

type stopTests struct{ suite.Suite }

func (s *stopTests) TestStop() {
	ctx := logger.NewContext(context.Background(), zaptest.NewLogger(s.T()))
	srv, err := NewServer(ctx)
	s.NoError(err)
	s.NoError(srv.stop())
	s.ErrorIs(srv.ctx.Err(), context.Canceled)
}

func TestRun(t *testing.T) {
	suite.Run(t, new(srvRunTests))
}

type srvRunTests struct {
	suite.Suite
	srv *Server
}

func (s *srvRunTests) SetupTest() {
	createRegistryKey = func() (bool, error) {
		return false, nil
	}

	h := new(heartbeatSerialTickerMock)
	h.On("Start").Return(nil)
	createHeartbeatSerialTicker = func(ctx context.Context) (starter, error) {
		return h, nil
	}

	ctx := logger.NewContext(context.Background(), zaptest.NewLogger(s.T()))
	var err error
	s.srv, err = NewServer(ctx)
	s.NoError(err)
}

func (s *srvRunTests) TestRunAsServiceTriggered() {
	watchManagerEvents = func(name string, handler svc.Handler) error {
		s.Equal(ServiceName, name)
		s.NotNil(handler)

		return nil
	}

	s.NoError(s.srv.Run())
}
