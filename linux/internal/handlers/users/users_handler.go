package users

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/pkg/messages"
	"marketplace-yaga/pkg/serial"
	"os"
	"os/exec"
	"os/user"
	"path"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// handlerName contain name of that handler.
const handlerName = "users_handler"

// DefaultMetadataURL contain URL which polled for User change requests.
const DefaultMetadataURL = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/ssh-keys"

var ErrWrongSshKeyFormat = errors.New("expected key format user:key")
var ErrEmptyUserName = errors.New("user is empty")
var ErrUserDirBroken = errors.New("user dir is not directory")

// serialPort is interface for read or write to serial port.
var serialPort = serial.NewBlockingWriter()

// UserHandle is struct, that implements needed methods for MetadataChangeHandler interface.
type UserHandle struct{}

// NewUserHandle return instance of UserHandle.
func NewUserHandle() *UserHandle {
	return &UserHandle{}
}

// String returns name of handler.
func (h *UserHandle) String() string {
	return handlerName
}

// Handle passes 'User change or creation' request to 'processRequest' function and writes result to serial port.
func (h *UserHandle) Handle(ctx context.Context, data []byte) {
	err := ctx.Err()
	logger.DebugCtx(ctx, err, "checked deadline or context cancellation")
	if err != nil {
		return
	}

	var resp response
	resp, err = processRequest(ctx, data)
	logger.DebugCtx(ctx, err, "processed request")
	// wont spam to serial port on equal requests

	runtime.GC()
	debug.FreeOSMemory()

	// unwrap to get envelope
	var e = messages.NewEnvelope()
	e.WithTimestamp(time.Now()).WithType(UserUpdateSshKeysResponseType)

	err = serialPort.WriteJSON(e.Wrap(resp))
	logger.DebugCtx(ctx, err, "writing to serial port",
		zap.String("response", fmt.Sprint(resp)),
		zap.String("envelope", fmt.Sprint(e)))
	if err != nil {
		return
	}
}

// processRequest unmarshalls passed data in request struct and checks  for validity.
// If request is valid and idempotent (we save sha256 hash) we pass it further to updateUsers function.
//nolint:nakedret
func processRequest(ctx context.Context, data []byte) (res response, err error) {
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

	users, err := parseSshKeys(data)
	logger.DebugCtx(ctx, err, "parsing users from metadata")
	if err != nil {
		return
	}
	err = updateUsers(ctx, users)
	if err != nil {
		return
	}

	res.withSuccess().withUsers(users)

	return
}

type User struct {
	Name   string
	SshKey string
}

func parseSshKeys(data []byte) ([]User, error) {
	var users []User
	userKeys := string(data)
	for _, line := range strings.Split(userKeys, "\n") {
		line := strings.Trim(line, " ")
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			return nil, ErrWrongSshKeyFormat
		}
		name := parts[0]
		if name == "" {
			return nil, ErrEmptyUserName
		}
		sshKey := parts[1]
		users = append(users, User{Name: name, SshKey: sshKey})
	}

	return users, nil
}

func updateUsers(ctx context.Context, users []User) error {
	var err error
	logger.DebugCtx(ctx, err, "checked deadline or context cancellation")
	if err = ctx.Err(); err != nil {
		return err
	}

	for _, u := range users {
		err = changeOrCreateUser(ctx, u.Name, u.SshKey)
		if err != nil {
			return err
		}
	}

	return nil
}

func changeOrCreateUser(ctx context.Context, username string, sshKey string) error {
	err := ensureUser(ctx, username)
	if err != nil {
		return err
	}
	userSshDir := path.Join("/home", username, ".ssh")
	info, err := os.Stat(userSshDir)
	if errors.Is(err, os.ErrNotExist) {
		logger.DebugCtx(ctx, err, "no user .ssh dir",
			zap.String("username", username))
		err := os.Mkdir(userSshDir, 0700)
		logger.DebugCtx(ctx, err, "created user .ssh dir",
			zap.String("username", username))
		if err != nil {
			return err
		}
	} else {
		if !info.IsDir() {
			return fmt.Errorf("%s .ssh dir is file", username)
		}
	}
	authorizedKeysFile := path.Join(userSshDir, "authorized_keys")

	_, err = os.Stat(authorizedKeysFile)
	if errors.Is(err, os.ErrNotExist) {
		_, err := os.Create(authorizedKeysFile)
		if err != nil {
			return err
		}
		logger.DebugCtx(ctx, err, "created user authorized_keys file",
			zap.String("username", username))
	}

	readFile, err := os.Open(authorizedKeysFile)
	if err != nil {
		return err
	}
	fileScanner := bufio.NewScanner(readFile)

	fileScanner.Split(bufio.ScanLines)
	found := false
	var lines []string
	for fileScanner.Scan() {
		line := fileScanner.Text()
		if line == sshKey {
			found = true
		}
		lines = append(lines, line)
	}

	err = readFile.Close()
	if err != nil {
		return err
	}
	if !found {
		lines = append(lines, sshKey)
		logger.DebugCtx(ctx, err, "added key for user",
			zap.String("username", username),
			zap.String("sshKey", sshKey),
		)
		err = os.WriteFile(authorizedKeysFile, []byte(strings.Join(lines, "\n")+"\n"), 0600)
		if err != nil {
			return err
		}
	}
	err = chown(userSshDir, username)
	if err != nil {
		return err
	}
	err = chown(authorizedKeysFile, username)
	if err != nil {
		return err
	}
	return nil
}

func ensureUser(ctx context.Context, username string) error {
	logger.DebugCtx(ctx, nil, "ensure user",
		zap.String("username", username))
	info, err := os.Stat(path.Join("/home", username))
	logger.DebugCtx(ctx, err, "check user home dir",
		zap.String("username", username))
	if errors.Is(err, os.ErrNotExist) {
		argUser := []string{"-m", username}
		userCmd := exec.Command("useradd", argUser...)
		if _, err := userCmd.Output(); err != nil {
			return err
		}
		logger.DebugCtx(ctx, err, "created user",
			zap.String("username", username))
	} else {
		if !info.IsDir() {
			return ErrUserDirBroken
		}
	}
	logger.DebugCtx(ctx, err, "user exists",
		zap.String("username", username))
	return nil
}

func chown(file, username string) error {
	group, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("error looking up %s user user info", username)
	}
	uid, _ := strconv.Atoi(group.Uid)
	gid, _ := strconv.Atoi(group.Gid)

	err = syscall.Chown(file, uid, gid)
	return err
}
