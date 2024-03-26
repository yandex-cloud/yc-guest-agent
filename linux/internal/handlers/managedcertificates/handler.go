package managedcertificates

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"marketplace-yaga/linux/internal/cm"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/pkg/messages"
	"marketplace-yaga/pkg/serial"
	"runtime"
	"runtime/debug"
	"time"
)

// handlerName contain name of that handler.
const handlerName = "managed_certificates_handler"

// DefaultMetadataURL contain URL which polled for Managed Certificates to file mapping.
const DefaultMetadataURL = "http://169.254.169.254/computeMetadata/v1/instance/attributes/managed-certificates"

// serialPort is interface for read or write to serial port.
var serialPort = serial.NewBlockingWriter()

// ManagedCertificatesHandler is struct, that implements needed methods for MetadataChangeHandler interface.
type ManagedCertificatesHandler struct{}

// CertificatesHandler return instance of ManagedCertificatesHandler.
func CertificatesHandler() *ManagedCertificatesHandler {
	return &ManagedCertificatesHandler{}
}

// String returns name of handler.
func (h *ManagedCertificatesHandler) String() string {
	return handlerName
}

var lastProcessedSha []byte

// Handle passes mapping of Managed Certificates on file paths to 'process' function and writes result to serial port.
func (h *ManagedCertificatesHandler) Handle(ctx context.Context, data []byte) {
	err := ctx.Err()
	if err != nil {
		logger.ErrorCtx(ctx, err, "checked deadline or context cancellation")
		return
	}
	dataSha := sha256.Sum256(data)
	if bytes.Compare(dataSha[:], lastProcessedSha) == 0 {
		return
	}

	var resp response
	resp, err = process(ctx, data)
	if err != nil {
		logger.ErrorCtx(ctx, err, "processed request")
	}

	runtime.GC()
	debug.FreeOSMemory()

	// unwrap to get envelope
	var e = messages.NewEnvelope()
	e.WithTimestamp(time.Now()).WithType(ManagedCertificatesResponseType)

	err = serialPort.WriteJSON(e.Wrap(resp))
	if err != nil {
		logger.ErrorCtx(ctx, err, "writing to serial port",
			zap.String("response", fmt.Sprint(resp)),
			zap.String("envelope", fmt.Sprint(e)))
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
	if err != nil {
		logger.ErrorCtx(ctx, err, "checked deadline or context cancellation")
		return
	}

	mngr := cm.New(ctx)
	msg, err := parse(data)
	if err != nil {
		logger.ErrorCtx(ctx, err, "parsing certificates from metadata")
		return
	}

	files, err := mngr.HandleCertificates(msg)
	if err != nil {
		return response{}, err
	}

	res.withSuccess().withCertificates(files)

	return
}

func parse(data []byte) (cm.CertificateMetadataMessage, error) {
	var msg cm.CertificateMetadataMessage
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}
