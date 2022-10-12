package lockboxsecrets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"go.uber.org/zap"
	"marketplace-yaga/linux/internal/lockbox"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/pkg/messages"
	"marketplace-yaga/pkg/serial"
	"runtime"
	"runtime/debug"
	"time"
)

// handlerName contain name of that handler.
const handlerName = "lockbox_secrets_handler"

// DefaultMetadataURL contain URL which polled for User change requests.
const DefaultMetadataURL = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/lockbox-secrets"

// serialPort is interface for read or write to serial port.
var serialPort = serial.NewBlockingWriter()

// LockboxHandler is struct, that implements needed methods for MetadataChangeHandler interface.
type LockboxHandler struct{}

// NewLockboxHandler return instance of LockboxHandler.
func NewLockboxHandler() *LockboxHandler {
	return &LockboxHandler{}
}

// String returns name of handler.
func (h *LockboxHandler) String() string {
	return handlerName
}

var lastProcessedSha []byte

// Handle passes mapping of Lockbox secrets on file paths to 'process' function and writes result to serial port.
func (h *LockboxHandler) Handle(ctx context.Context, data []byte) {
	err := ctx.Err()
	logger.DebugCtx(ctx, err, "checked deadline or context cancellation")
	if err != nil {
		return
	}
	dataSha := sha256.Sum256(data)
	if bytes.Compare(dataSha[:], lastProcessedSha) == 0 {
		return
	}

	var resp response
	resp, err = process(ctx, data)
	logger.DebugCtx(ctx, err, "processed request")
	// wont spam to serial port on equal requests

	runtime.GC()
	debug.FreeOSMemory()

	// unwrap to get envelope
	var e = messages.NewEnvelope()
	e.WithTimestamp(time.Now()).WithType(LockboxSecretsResponseType)

	err = serialPort.WriteJSON(e.Wrap(resp))
	logger.DebugCtx(ctx, err, "writing to serial port",
		zap.String("response", fmt.Sprint(resp)),
		zap.String("envelope", fmt.Sprint(e)))
	if err != nil {
		return
	}
	lastProcessedSha = dataSha[:]
}

//nolint:nakedret
func process(ctx context.Context, data []byte) (res response, err error) {
	defer func() {
		if err != nil {
			res.withError(err)
		}
	}()

	err = ctx.Err()
	logger.DebugCtx(ctx, err, "checked deadline or context cancellation")
	if err != nil {
		return
	}

	mngr := lockbox.New(ctx)
	msg, err := mngr.Parse(data)
	logger.DebugCtx(ctx, err, "parsing users from metadata")
	if err != nil {
		return
	}

	files, err := mngr.HandleSecrets(msg)
	if err != nil {
		return response{}, err
	}

	res.withSuccess().withFiles(files)

	return
}