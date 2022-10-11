package logger

import (
	"context"
	"marketplace-yaga/pkg/messages"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

func TestNewLogger(t *testing.T) {
	suite.Run(t, new(newLoggerTests))
}

type newLoggerTests struct{ suite.Suite }

func (s *newLoggerTests) TestNewLogger() {
	tests := []struct {
		lvl        string
		withSerial bool
		wantErr    bool
	}{
		{lvl: "operationcwal", withSerial: false, wantErr: true},
		{lvl: "operationcwal", withSerial: true, wantErr: true},
		{lvl: "Info", withSerial: false, wantErr: false},
		{lvl: "Info", withSerial: true, wantErr: false},
		{lvl: "Debug", withSerial: false, wantErr: false},
		{lvl: "Debug", withSerial: true, wantErr: false},
	}

	for _, t := range tests {
		l, err := NewLogger(t.lvl, t.withSerial)
		if t.wantErr {
			s.Error(err)
			s.Nil(l)
		} else {
			s.NoError(err)
			var zp *zap.Logger
			s.IsType(zp, l)
		}
	}
}

func TestMustNewLogger(t *testing.T) {
	suite.Run(t, new(mustNewLoggerTests))
}

type mustNewLoggerTests struct{ suite.Suite }

func TestContext(t *testing.T) {
	suite.Run(t, new(contextTests))
}

type contextTests struct{ suite.Suite }

func (s *contextTests) TestLContextNoopIfNotStored() {
	l := FromContext(context.Background())
	s.Equal(zap.NewNop(), l)
}

func (s *contextTests) TestLContextNoopFromNil() {
	//nolint:SA1012
	l := FromContext(nil)
	s.Equal(zap.NewNop(), l)
}

func (s *contextTests) TestLContextMustSucceed() {
	l, err := NewLogger("Info", true)
	s.NoError(err)
	var zp *zap.Logger
	s.IsType(zp, l)

	ctx := NewContext(context.Background(), l)
	s.Equal(FromContext(ctx), l)
}

type serialMock struct{ mock.Mock }

func (m *serialMock) Write(b []byte) (int, error) {
	args := m.Called(b)

	return args.Int(0), args.Error(1)
}

func (m *serialMock) WriteJSON(j interface{}) error {
	args := m.Called(j)

	return args.Error(0)
}

func (m *serialMock) Close() error {
	args := m.Called()

	return args.Error(0)
}

func TestLogWriter(t *testing.T) {
	suite.Run(t, new(logWriterTests))
}

type logWriterTests struct{ suite.Suite }

func (s *logWriterTests) TestSerialWriter() {
	p := new(serialMock)
	p.On("Write", mock.Anything).Return(0, nil)
	serialPort = p

	j := new(serialJSONWriter)
	_, err := j.Write([]byte(`{"field":"ophelia"}`))
	s.NoError(err)

	bs, ok := p.Calls[0].Arguments.Get(0).([]byte)
	s.True(ok)

	type payload struct{ Field string }
	var pl payload
	s.NoError(messages.UnmarshalPayload(bs, &pl))
	s.Equal(payload{Field: "ophelia"}, pl)
}

func (s *logWriterTests) TestSerialWriterError() {
	p := new(serialMock)
	p.On("Write", mock.Anything).Return(0, assert.AnError)
	serialPort = p

	j := new(serialJSONWriter)
	_, err := j.Write([]byte(`{"field":"ophelia"}`))
	s.Error(err)
}

func TestLogging(t *testing.T) {
	suite.Run(t, new(loggingTests))
}

type loggingTests struct {
	suite.Suite
	p                *serialMock
	ctxInfo          context.Context
	ctxDebug         context.Context
	ctxInfoNoSerial  context.Context
	ctxDebugNoSerial context.Context
}

func (s *loggingTests) SetupTest() {
	s.p = new(serialMock)
	s.p.On("Write", mock.Anything).Return(0, nil)
	serialPort = s.p

	i, _ := NewLogger("Info", true)
	s.ctxInfo = NewContext(context.Background(), i)

	d, _ := NewLogger("Debug", true)
	s.ctxDebug = NewContext(context.Background(), d)

	is, _ := NewLogger("Info", false)
	s.ctxInfoNoSerial = NewContext(context.Background(), is)

	ds, _ := NewLogger("Debug", false)
	s.ctxDebugNoSerial = NewContext(context.Background(), ds)
}

type logMsg struct {
	Msg   string
	Level string
}

func (s *loggingTests) TestInfoEnvelopeHasType() {
	InfoCtx(s.ctxInfo, nil, "radiofreezerg")

	bs, ok := s.p.Calls[0].Arguments.Get(0).([]byte)
	s.True(ok)

	e, err := messages.UnmarshalEnvelope(bs)
	s.NoError(err)
	s.IsType(messages.NewEnvelope(), e)
	s.Equal("log", e.Type)
}

func (s *loggingTests) TestDebugEnvelopeHasType() {
	DebugCtx(s.ctxDebug, nil, "radiofreezerg")

	bs, ok := s.p.Calls[0].Arguments.Get(0).([]byte)
	s.True(ok)

	e, err := messages.UnmarshalEnvelope(bs)
	s.NoError(err)
	s.IsType(messages.NewEnvelope(), e)
	s.Equal("log", e.Type)
}

func (s *loggingTests) TestCatchInfoFromSerial() {
	InfoCtx(s.ctxInfo, nil, "radiofreezerg")

	bs, ok := s.p.Calls[0].Arguments.Get(0).([]byte)
	s.True(ok)

	var m logMsg
	s.NoError(messages.UnmarshalPayload(bs, &m))
	s.Equal("radiofreezerg", m.Msg)
	s.Equal("INFO", m.Level)
}

func (s *loggingTests) TestCatchDebugFromSerial() {
	DebugCtx(s.ctxDebug, nil, "radiofreezerg")

	bs, ok := s.p.Calls[0].Arguments.Get(0).([]byte)
	s.True(ok)

	var m logMsg
	s.NoError(messages.UnmarshalPayload(bs, &m))
	s.Equal("radiofreezerg", m.Msg)
	s.Equal("DEBUG", m.Level)
}

func (s *loggingTests) TestCatchInfoFromDebug() {
	InfoCtx(s.ctxInfo, nil, "radiofreezerg")

	bs, ok := s.p.Calls[0].Arguments.Get(0).([]byte)
	s.True(ok)

	var m logMsg
	s.NoError(messages.UnmarshalPayload(bs, &m))
	s.Equal("radiofreezerg", m.Msg)
	s.Equal("INFO", m.Level)
}

func (s *loggingTests) TestNoDebugFromInfo() {
	DebugCtx(s.ctxInfo, nil, "radiofreezerg")
	s.p.AssertNotCalled(s.T(), "Write")
}

func (s *loggingTests) TestNoInfoFromDisabledSerial() {
	InfoCtx(s.ctxInfoNoSerial, nil, "radiofreezerg")
	s.p.AssertNotCalled(s.T(), "Write")
}

func (s *loggingTests) TestNoDebugFromDisabledSerial() {
	DebugCtx(s.ctxDebugNoSerial, nil, "radiofreezerg")
	s.p.AssertNotCalled(s.T(), "Write")
}

// condLog
