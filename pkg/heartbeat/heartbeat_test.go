package heartbeat

import (
	"context"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/pkg/messages"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
)

type serialPortMock struct {
	mock.Mock
}

func (m *serialPortMock) Write(b []byte) (int, error) {
	args := m.Called(b)

	return args.Int(0), args.Error(1)
}

func (m *serialPortMock) WriteJSON(j interface{}) error {
	args := m.Called(j)

	return args.Error(0)
}

func (m *serialPortMock) Close() error {
	args := m.Called()

	return args.Error(0)
}

func TestSerialReporter(t *testing.T) {
	suite.Run(t, new(serialReporterPipeline))
}

type serialReporterPipeline struct {
	suite.Suite
	p      *serialPortMock
	ctx    context.Context
	cancel context.CancelFunc
}

func (s *serialReporterPipeline) SetupTest() {
	ctx := logger.NewContext(context.Background(), zaptest.NewLogger(s.T()))
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.p = new(serialPortMock)
	s.p.On("WriteJSON", mock.Anything).Return(nil)
	serialPort = s.p
}

func (s *serialReporterPipeline) TeardownTest() {
	s.cancel()
}

const waitTime = 1 * time.Second

func (s *serialReporterPipeline) TestReporterPipeline() {
	h, err := NewSerialTicker(s.ctx)
	s.NoError(err)
	s.NotEmpty(h)
	s.NoError(h.Start())

	<-time.After(waitTime)

	m, ok := s.p.Calls[0].Arguments.Get(0).(messages.Message)
	s.True(ok)
	s.NotEqual(messages.Message{}, m)

	var hb status
	hb, ok = m.Payload.(status)
	s.True(ok)
	s.NotEqual(status{}, hb)
	s.Equal("ok", hb.Status)
}

func (s *serialReporterPipeline) TestNoCallsAfterCancelOfContext() {
	h, err := NewSerialTicker(s.ctx)
	s.NoError(err)
	s.NotEmpty(h)
	s.NoError(h.Start())

	<-time.After(waitTime)
	s.cancel()
	<-time.After(waitTime)
	before := len(s.p.Calls)
	<-time.After(waitTime)
	after := len(s.p.Calls)

	s.Equal(before, after)
}

func (s *serialReporterPipeline) TestStartWithCanceledContext() {
	h, err := NewSerialTicker(s.ctx)
	s.NoError(err)
	s.NotEmpty(h)

	s.cancel()
	s.ErrorIs(h.Start(), context.Canceled)
}
