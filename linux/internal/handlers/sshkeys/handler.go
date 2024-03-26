package sshkeys

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"marketplace-yaga/linux/internal/usermanager"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/pkg/messages"
	"marketplace-yaga/pkg/serial"
	"os/user"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// handlerName contain name of that handler.
const handlerName = "ssh_keys_handler"

// DefaultMetadataURL contain URL which polled for User change requests.
const DefaultMetadataURL = "http://169.254.169.254/computeMetadata/v1/instance/attributes/ssh-keys"

var ErrWrongSshKeyFormat = errors.New("expected key format user:key")
var ErrEmptyUserName = errors.New("user is empty")

// serialPort is interface for read or write to serial port.
var serialPort = serial.NewBlockingWriter()

// UserHandler is struct, that implements needed methods for MetadataChangeHandler interface.
type UserHandler struct{}

// NewUserHandler return instance of UserHandler.
func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

// String returns name of handler.
func (h *UserHandler) String() string {
	return handlerName
}

var lastProcessedSha []byte

// Handle passes 'User change or creation' request to 'processRequest' function and writes result to serial port.
func (h *UserHandler) Handle(ctx context.Context, data []byte) {
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
	resp, err = processRequest(ctx, data)
	if err != nil {
		logger.ErrorCtx(ctx, err, "processed request")
	}
	// wont spam to serial port on equal requests

	runtime.GC()
	debug.FreeOSMemory()

	// unwrap to get envelope
	var e = messages.NewEnvelope()
	e.WithTimestamp(time.Now()).WithType(UserUpdateSshKeysResponseType)

	err = serialPort.WriteJSON(e.Wrap(resp))
	if err != nil {
		logger.ErrorCtx(ctx, err, "writing to serial port",
			zap.String("response", fmt.Sprint(resp)),
			zap.String("envelope", fmt.Sprint(e)))
		return
	}
	lastProcessedSha = dataSha[:]
}

// processRequest unmarshalls passed data in request struct and checks  for validity.
//
//nolint:nakedret
func processRequest(ctx context.Context, data []byte) (res response, err error) {
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

	parsedUsers, err := parseSshKeys(data)
	if err != nil {
		logger.ErrorCtx(ctx, err, "parsing users from metadata")
		return
	}
	mngr := usermanager.New(ctx)

	for _, u := range parsedUsers {
		err = mngr.ValidateUsername(u.Name)
		if err != nil {
			return
		}
		err = mngr.ValidateUser(u.Name)
		if err != nil {
			return
		}
		_, err = mngr.Exist(u.Name)
		if err != nil {
			err = mngr.CreateUser(u.Name)
			if err != nil {
				return
			}

		}
		sysUser, _ := user.Lookup(u.Name)

		err = mngr.AddSshKey(sysUser, u.SshKey)
		if err != nil {
			return
		}
	}

	res.withSuccess().withUsers(parsedUsers)

	return
}

func parseSshKeys(data []byte) ([]usermanager.User, error) {
	var users []usermanager.User
	userKeys := string(data)
	for _, line := range strings.Split(userKeys, "\n") {
		line := strings.Trim(line, " ")
		if len(line) == 0 {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			return nil, ErrWrongSshKeyFormat
		}
		name := parts[0]
		if name == "" {
			return nil, ErrEmptyUserName
		}
		sshKey := parts[1]
		users = append(users, usermanager.User{Name: name, SshKey: sshKey})
	}

	return users, nil
}
