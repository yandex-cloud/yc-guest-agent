package users

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/spf13/afero"
	"marketplace-yaga/linux/internal/usermanager"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/pkg/messages"
	"marketplace-yaga/pkg/passwords"
	"marketplace-yaga/pkg/serial"
	"math/big"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"go.uber.org/zap"
)

// handlerName contain name of that handler.
const handlerName = "users_handler"

// DefaultMetadataURL contain URL which polled for user change requests.
const DefaultMetadataURL = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/linux-users"

// ErrIdemp is returned when hash of user change request already in registry.
var ErrIdemp = errors.New("operation already performed")

// ErrTimeFrame is returned if request timeframe out of bound: -requestTimeframe, time.Now(), +requestTimeframe.
var ErrTimeFrame = errors.New("request timestamp is out of allowed timeframe")

// requestTimeframe is an interval from time.now() at which request will be considered valid.
const requestTimeframe = time.Minute * 5

// idempotencyFile is file path where we'll use store idempotency key.
const idempotencyFile = `/opt/yandex-guest-agent/idempotency`

// passwordLength is length of generated password.
const passwordLength = uint(15)

// passwordNumSymbols is number of symbols in generated password.
const passwordNumSymbols = uint(5)

// passwordNumDigits is number of symbols in generated password.
const passwordNumDigits = uint(3)

// passwordNoUppers restricts use of upper case letters.
const passwordNoUppers = false

// passwordLowerLetters is override for lower letters pool.
const passwordLowerLetters = "" // "" to use defaults

// passwordLowerLetters is override for upper letters pool.
const passwordUpperLetters = "" // "" to use defaults

// passwordLowerLetters is override for digits pool.
const passwordDigits = "" // "" to use defaults

// passwordLowerLetters is override for symbols pool.
const passwordSymbols = "" // "" to use defaults

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

// Handle passes 'user change or creation' request to 'processRequest' function and writes result to serial port.
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
	if errors.Is(err, ErrIdemp) {
		return
	}

	runtime.GC()
	debug.FreeOSMemory()

	// unwrap to get envelope
	var e *messages.Envelope
	e, err = messages.UnmarshalEnvelope(data)
	logger.DebugCtx(ctx, err, "unwrap envelope from message")
	if err != nil {
		return
	}
	e.WithTimestamp(time.Now()).WithType(UserChangeResponseType)

	err = serialPort.WriteJSON(e.Wrap(resp))
	logger.DebugCtx(ctx, err, "writing to serial port",
		zap.String("response", fmt.Sprint(resp)),
		zap.String("envelope", fmt.Sprint(e)))
	if err != nil {
		return
	}
}

// processRequest unmarshalls passed data in request struct and checks  for validity.
// If request is valid and idempotent (we save sha256 hash) we pass it further to changeOrCreateUser function.
//
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

	var req request
	err = messages.UnmarshalPayload(data, &req)
	logger.DebugCtx(ctx, err, "unmarshal request from message payload")
	if err != nil {
		return
	}
	res.withRequest(req)

	rm := NewRequestManager(afero.NewOsFs())
	var hash string
	hash, err = rm.GetSHA256(req)
	logger.DebugCtx(ctx, err, "hashed request")
	if err != nil {
		return
	}

	err = rm.ValidateRequestHash(hash)
	logger.DebugCtx(ctx, err, "checked request hash for idempotency", zap.String("hash", hash))
	if err != nil {
		return
	}

	err = os.WriteFile(idempotencyFile, []byte(hash), 0600)
	logger.DebugCtx(ctx, err, "saved request hash to file",
		zap.String("idempotencyFile", idempotencyFile),
		zap.String("hash", hash))
	if err != nil {
		return
	}

	err = rm.ValidateRequestTimestamp(req.Expires)
	logger.DebugCtx(ctx, err, "validated user request timestamp",
		zap.String("request", fmt.Sprint(req)))
	if err != nil {
		return
	}

	um := usermanager.New(ctx)
	var encPwd string
	encPwd, err = changeOrCreateUser(ctx, um, req)
	logger.DebugCtx(ctx, err, "changed or created user",
		zap.String("request", fmt.Sprint(req)))
	if err != nil {
		return
	}
	res.withSuccess().withEncryptedPassword(encPwd)

	return
}

// userManagerProvider is an interface that describes needed methods to manage users.
type userManagerProvider interface {
	Exist(username string) (bool, error)
	SetPassword(username, password string) (err error)
	CreateUser(username string) error
	AddToAdministrators(username string) error
}

// changeOrCreateUser creates local user if one in request could not be found or resets password for existing one.
// As a result passes back encrypted password with the public provided in request. (via Modulus and Exponent).
//
//nolint:nakedret
func changeOrCreateUser(ctx context.Context, userManager userManagerProvider, req request) (encPwd string, err error) {
	logger.DebugCtx(ctx, err, "checked deadline or context cancellation")
	if err = ctx.Err(); err != nil {
		return
	}

	var pwd string
	pwd, err = pwdGen.Generate(passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers)
	logger.DebugCtx(ctx, err, "generated password")
	if err != nil {
		return
	}

	encPwd, err = encryptPassword(req.Modulus, req.Exponent, pwd)
	logger.DebugCtx(ctx, err, "encrypted password",
		zap.String("encryptedPassword", encPwd),
		zap.String("Modulus", req.Modulus),
		zap.String("Exponent", req.Exponent))
	if err != nil {
		return
	}

	var exist bool
	exist, err = userManager.Exist(req.Username)
	logger.DebugCtx(ctx, err, "checked user existence",
		zap.String("username", req.Username),
		zap.String("exist", fmt.Sprint(exist)))

	if exist {
		err = userManager.SetPassword(req.Username, pwd)
		logger.DebugCtx(ctx, err, "reset password",
			zap.String("username", req.Username))
		if err != nil {
			return
		}
	} else {
		err = userManager.CreateUser(req.Username)
		logger.DebugCtx(ctx, err, "created user",
			zap.String("username", req.Username))
		if err != nil {
			return
		}

		err = userManager.SetPassword(req.Username, pwd)
		logger.DebugCtx(ctx, err, "reset password",
			zap.String("username", req.Username))
		if err != nil {
			return
		}

		err = userManager.AddToAdministrators(req.Username)
		logger.DebugCtx(ctx, err, "add to administrators",
			zap.String("username", req.Username))
		if err != nil {
			return
		}
	}

	return
}

// pwdGen is interface that generates passwords.
var pwdGen = passwords.NewGenerator(passwordLowerLetters, passwordUpperLetters, passwordDigits, passwordSymbols)

// encryptPassword encrypts password with the public provided in request (via modulus and exponent).
func encryptPassword(mod, exp, pwd string) (encPwd string, err error) {
	m, err := b64strToBigInt(mod)
	if err != nil {
		return
	}

	e, err := b64strToBigInt(exp)
	if err != nil {
		return
	}

	rsaKey := &rsa.PublicKey{N: m, E: int(e.Int64())}
	enc, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaKey, []byte(pwd), nil)
	if err != nil {
		return
	}

	encPwd = base64.StdEncoding.EncodeToString(enc)

	return
}

// b64strToBigInt is function which convert base64 string to big.Int.
func b64strToBigInt(s string) (b *big.Int, err error) {
	bs, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return
	}

	b = big.NewInt(0).SetBytes(bs)

	return
}
