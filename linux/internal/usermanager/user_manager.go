package usermanager

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/GehirnInc/crypt/sha512_crypt"
	"github.com/spf13/afero"
	"go.uber.org/zap"
	"io/fs"
	"marketplace-yaga/linux/internal/executor"
	"marketplace-yaga/linux/internal/executor/argument"
	"marketplace-yaga/linux/internal/executor/command"
	"marketplace-yaga/pkg/logger"
	"math/big"
	"os"
	"os/user"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type User struct {
	Name   string `json:"name"`
	SshKey string `json:"sshKey"`
}

type Manager struct {
	ctx      context.Context
	fs       afero.Fs
	executor executor.ExecutorService
}

var ErrRestrictedUser = errors.New(`modifications to restricted users not allowed (BUILTIN\Administrator on windows or system users on linux)`)

func New(ctx context.Context) *Manager {
	return newManager(ctx)
}
func newManager(ctx context.Context) *Manager {
	// could be pushed up into config later
	const commandsTimeout = 10 * time.Second

	// propagate context so if server signaled to stop, commands in flight also canceled
	osexec := executor.NewBuilder(ctx).WithTimeout(commandsTimeout).Build()

	return &Manager{
		ctx:      ctx,
		fs:       afero.NewOsFs(),
		executor: osexec,
	}
}

func (m *Manager) GetLocalNonSystemUsers() ([]string, error) {
	logger.DebugCtx(m.ctx, nil, "get local usernames")

	var users []string
	usersCollector := func(username string) error {
		users = append(users, username)
		return nil
	}

	if err := parseUsernames(m.fs, usersCollector); err != nil {
		return nil, err
	}

	return users, nil
}

func (m *Manager) ValidateUser(username string) error {
	logger.DebugCtx(m.ctx, nil, "validate username",
		zap.String("username", username))

	if err := validateUsernamePattern(username); err != nil {
		return err
	}

	return validateIsNotSystemUser(username)
}

func (m *Manager) ValidateUsername(username string) error {
	return validateUsernamePattern(username)
}

func (m *Manager) CreateUser(username string) error {
	logger.DebugCtx(m.ctx, nil, "create user",
		zap.String("username", username))

	cmd, err := command.New(
		argument.New("useradd"),
		argument.New("--create-home"),
		argument.New(username))
	if err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}

	return m.executor.Run(cmd)
}

func (m *Manager) AddSshKey(u *user.User, sshKey string) (err error) {
	if err != nil {
		return err
	}
	userSshDir, err := m.ensureSshFolder(u.HomeDir, err)
	if err != nil {
		return err
	}

	authorizedKeysFile, err := m.ensureAuthorizedKeysFile(u.HomeDir, userSshDir)
	if err != nil {
		return err
	}

	err = m.appendKey(u.HomeDir, sshKey, err, authorizedKeysFile)
	if err != nil {
		return err
	}

	err = m.chown(userSshDir, u)
	if err != nil {
		return err
	}

	err = m.chown(authorizedKeysFile, u)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) ensureAuthorizedKeysFile(username string, userSshDir string) (string, error) {
	authorizedKeysFile := path.Join(userSshDir, "authorized_keys")

	_, err := m.fs.Stat(authorizedKeysFile)
	if errors.Is(err, fs.ErrNotExist) {
		_, err := m.fs.Create(authorizedKeysFile)
		if err != nil {
			logger.ErrorCtx(m.ctx, err, "create user authorized_keys file",
				zap.String("username", username))
			return "", err
		}
		logger.DebugCtx(m.ctx, nil, "created user authorized_keys file",
			zap.String("username", username))
	}
	return authorizedKeysFile, nil
}

func (m *Manager) ensureSshFolder(homedir string, err error) (string, error) {
	userSshDir := path.Join(homedir, ".ssh")
	info, err := m.fs.Stat(userSshDir)
	if errors.Is(err, fs.ErrNotExist) {
		logger.ErrorCtx(m.ctx, err, "no user .ssh dir",
			zap.String("homedir", homedir))
		err := m.fs.Mkdir(userSshDir, 0700)
		if err != nil {
			logger.ErrorCtx(m.ctx, err, "created user .ssh dir",
				zap.String("homedir", homedir))
			return "", err
		}
	} else {
		if !info.IsDir() {
			return "", fmt.Errorf("%s .ssh dir is file", homedir)
		}
	}
	return userSshDir, nil
}

func (m *Manager) appendKey(homedir string, sshKey string, err error, authorizedKeysFile string) error {
	readFile, err := m.fs.Open(authorizedKeysFile)
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
		logger.ErrorCtx(m.ctx, err, "add key for user",
			zap.String("homedir", homedir),
			zap.String("sshKey", sshKey),
		)
		return err
	}
	if !found {
		lines = append(lines, sshKey)
		file, err := m.fs.OpenFile(authorizedKeysFile, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return err
		}

		_, err = file.Write([]byte(strings.Join(lines, "\n") + "\n"))
		if err != nil {
			logger.ErrorCtx(m.ctx, err, "written authorized_keys",
				zap.String("homedir", homedir),
				zap.String("sshKey", sshKey),
			)
			return err
		}
		err = file.Close()
		if err != nil {
			return err
		}
	}
	return err
}
func (m *Manager) Exist(username string) (bool, error) {
	_, err := user.Lookup(username)
	if err != nil {
		return false, fmt.Errorf("failed to lookup user: %w", err)
	}

	return true, nil
}

func (m *Manager) chown(file string, u *user.User) error {
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	return m.fs.Chown(file, uid, gid)
}

func validateIsNotSystemUser(username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return nil
	}

	id, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("failed to convert uid (%v) to int: %w", u.Uid, err)
	}

	if id < 1000 {
		return ErrRestrictedUser
	}

	return nil
}

func validateUsernamePattern(username string) error {
	// a bit more strict, but xkcd.com/1171/
	expr := `^[a-zA-Z][a-zA-Z0-9_.-]{0,62}[a-zA-Z0-9]$`
	re, err := regexp.Compile(expr)
	if err != nil {
		return err
	}

	if ok := re.MatchString(username); !ok {
		return fmt.Errorf("failed to validate username: %v, using expression: %v", username, expr)
	}

	// check that name doesn't have consecutive dots
	// yes, not in regexp - to simplify reading
	if strings.Contains(username, "..") {
		return fmt.Errorf("failed to validate username: %v, consecutive dots not allowed", username)
	}

	return nil
}

func (m *Manager) SetPassword(username, password string) error {
	logger.DebugCtx(m.ctx, nil, "set password",
		zap.String("username", username))

	hashedPassword, err := getSHA512Crypt(password)
	if err != nil {
		return fmt.Errorf("failed to get sha 512 crypt hash: %w", err)
	}

	logger.InfoCtx(m.ctx, nil, "set password",
		zap.String("hash", hashedPassword))

	cmd, err := command.New(
		argument.New("usermod"),
		argument.New("--password"),
		argument.New(hashedPassword, argument.Sensitive(), argument.NoEscape()),
		argument.New(username))

	if err != nil {
		return fmt.Errorf("failed construct command: %w", err)
	}

	return m.executor.Run(cmd)
}

func (m *Manager) AddToAdministrators(username string) error {
	logger.DebugCtx(m.ctx, nil, `add to local group sudo`,
		zap.String("username", username))

	const group = "sudo"

	cmd, err := command.New(
		argument.New("usermod"),
		argument.New("--groups"),
		argument.New(group),
		argument.New(username))

	if err != nil {
		return fmt.Errorf("failed construct command: %w", err)
	}

	return m.executor.Run(cmd)
}

func getSHA512Crypt(password string) (string, error) {
	// 16 is limitation
	const maxSHA512SaltLength = 16
	s := newCryptSalt(maxSHA512SaltLength)

	// sha512crypt has well-known prefix
	const prefixSHA512crypt = `$6$`
	salt := fmt.Sprintf("%s%s", prefixSHA512crypt, s)

	hashedPassword, err := sha512_crypt.New().Generate([]byte(password), []byte(salt))
	if err != nil {
		return "", err
	}

	return hashedPassword, nil
}

func newCryptSalt(length uint) string {
	// accepted chars are [./0-9A-Za-z]
	const alphabet = `./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz`

	salt := ""
	for i := uint(0); i < length; i++ {
		salt += getRandomChar(alphabet)
	}

	return salt
}

func getRandomChar(s string) string {
	rs := []rune(s)
	l := len(rs)

	n, err := rand.Int(rand.Reader, big.NewInt(int64(l)))
	if err != nil {
		panic(err)
	}

	return string(rs[n.Int64()])
}
