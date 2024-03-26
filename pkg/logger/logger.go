package logger

import (
	"context"
	"encoding/json"
	"marketplace-yaga/pkg/messages"
	"marketplace-yaga/pkg/serial"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// serialJSONWriter is serial port writer, that can wrap log messages correctly without excessive escape chars.
type serialJSONWriter struct{}

func (serialJSONWriter) Write(d []byte) (n int, err error) {
	var bs []byte
	if bs, err = messages.NewEnvelope().WithType("log").Marshal(json.RawMessage(d)); err != nil {
		return
	}

	return serialPort.Write(append(bs, []byte("\n")...))
}

// noopSyncerCloser is noop wrapper to make zap.Sink from io.Writer.
type noopSyncerCloser struct{ *serialJSONWriter }

func (noopSyncerCloser) Sync() error  { return nil }
func (noopSyncerCloser) Close() error { return nil }

var serialPort = serial.NewBlockingWriter()

var defaultEncoderConfig = zapcore.EncoderConfig{
	MessageKey:     "msg",
	LevelKey:       "level",
	TimeKey:        "ts",
	CallerKey:      "caller",
	NameKey:        "name",
	StacktraceKey:  "strace",
	EncodeLevel:    zapcore.CapitalLevelEncoder,
	EncodeTime:     zapcore.ISO8601TimeEncoder,
	EncodeDuration: zapcore.StringDurationEncoder,
	EncodeCaller:   zapcore.ShortCallerEncoder,
}

// NewLogger create console and serial port loggers if specified.
// Serial log formatted as JSON's for easy-parsing.
// Console log utilize text encoder.
func NewLogger(lvl string, withSerial bool) (*zap.Logger, error) {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(lvl)); err != nil {
		return nil, err
	}

	cores := zapcore.NewCore(
		zapcore.NewConsoleEncoder(defaultEncoderConfig),
		zapcore.Lock(os.Stdout),
		level)

	if withSerial {
		sjw := zapcore.Lock(noopSyncerCloser{new(serialJSONWriter)})
		sje := zapcore.NewJSONEncoder(defaultEncoderConfig)

		cores = zapcore.NewTee(
			cores,
			zapcore.NewCore(sje, sjw, level))
	}

	return zap.New(cores), nil
}

type key int

var loggerKey key

// FromContext return logger which is stored in given context or noop logger if no logger is found.
func FromContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return zap.NewNop()
	}

	v := ctx.Value(loggerKey)
	if v == nil {
		return zap.NewNop()
	}

	return v.(*zap.Logger)
}

// NewContext creates context and stores logger in it.
func NewContext(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

const callerSkipNum = 2

var errOptions = []zap.Option{zap.AddCallerSkip(callerSkipNum), zap.AddCaller(), zap.AddStacktrace(zapcore.DebugLevel)}

func DebugCtx(ctx context.Context, err error, msg string, fields ...zap.Field) {
	l := FromContext(ctx)
	if err != nil {
		l = l.WithOptions(errOptions...)
		log(l.With(zap.Error(err)), msg, zapcore.ErrorLevel, fields...)
	} else {
		log(l.With(zap.Error(err)), msg, zapcore.DebugLevel, fields...)
	}
}

func ErrorCtx(ctx context.Context, err error, msg string, fields ...zap.Field) {
	l := FromContext(ctx)
	if err != nil {
		l = l.WithOptions(errOptions...)
	}

	log(l.With(zap.Error(err)), msg, zapcore.ErrorLevel, fields...)
}

func InfoCtx(ctx context.Context, err error, msg string, fields ...zap.Field) {
	log(FromContext(ctx).With(zap.Error(err)), msg, zapcore.InfoLevel, fields...)
}

//goland:noinspection GoUnusedExportedFunction
func FatalCtx(ctx context.Context, err error, msg string, fields ...zap.Field) {
	log(FromContext(ctx).With(zap.Error(err)), msg, zapcore.InfoLevel, fields...)
	os.Exit(1)
}

func log(l *zap.Logger, msg string, level zapcore.Level, fields ...zap.Field) {
	if ce := l.Check(level, msg); ce != nil {
		ce.Write(fields...)
	}
}
