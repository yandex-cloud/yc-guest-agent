package kmssecrets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"marketplace-yaga/linux/internal/kms"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/pkg/messages"
	"marketplace-yaga/pkg/serial"
	"runtime"
	"runtime/debug"
	"time"
)

// handlerName contain name of that handler.
const handlerName = "kms_secrets_handler"

// DefaultMetadataURL contain URL which polled for KMS encoded secrets to file mapping.
const DefaultMetadataURL = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/kms-secrets"

// serialPort is interface for read or write to serial port.
var serialPort = serial.NewBlockingWriter()

// KmsHandler is struct, that implements needed methods for MetadataChangeHandler interface.
type KmsHandler struct{}

// NewKmsHandler return instance of KmsHandler.
func NewKmsHandler() *KmsHandler {
	return &KmsHandler{}
}

// String returns name of handler.
func (h *KmsHandler) String() string {
	return handlerName
}

var lastProcessedSha []byte

// Handle passes KMS encoded secrets mapping on files to 'process' function and writes result to serial port.
func (h *KmsHandler) Handle(ctx context.Context, data []byte) {
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

	runtime.GC()
	debug.FreeOSMemory()

	// unwrap to get envelope
	var e = messages.NewEnvelope()
	e.WithTimestamp(time.Now()).WithType(KmsSecretsResponseType)

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

	mngr := kms.New(ctx)
	msg, err := parse(data)
	logger.DebugCtx(ctx, err, "parsing kms encrypted secrets from metadata")
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

func parse(data []byte) (kms.SecretMetadataMessage, error) {
	var msg kms.SecretMetadataMessage
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}
