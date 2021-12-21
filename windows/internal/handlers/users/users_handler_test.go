package users

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/pkg/messages"
	"marketplace-yaga/windows/internal/registry"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
)

type regKeyHandlerMock struct {
	mock.Mock
}

func (m *regKeyHandlerMock) ReadStringProperty(name string) (string, error) {
	args := m.Called(name)

	return args.String(0), args.Error(1)
}

func (m *regKeyHandlerMock) WriteStringProperty(name, value string) error {
	args := m.Called(name, value)

	return args.Error(0)
}

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

type userManagerMock struct {
	mock.Mock
}

func (m *userManagerMock) Exist(username string) (bool, error) {
	args := m.Called(username)

	return args.Bool(0), args.Error(1)
}

func (m *userManagerMock) SetPassword(username, password string) error {
	args := m.Called(username, password)

	return args.Error(0)
}

func (m *userManagerMock) CreateUser(username, password string) error {
	args := m.Called(username, password)

	return args.Error(0)
}

func (m *userManagerMock) AddToAdministrators(username string) error {
	args := m.Called(username)

	return args.Error(0)
}

type passwordGeneratorMock struct {
	mock.Mock
}

func (m *passwordGeneratorMock) Generate(length uint, numDigits uint, numSymbols uint, noUpper bool) (string, error) {
	args := m.Called(length, numDigits, numSymbols, noUpper)

	return args.String(0), args.Error(1)
}

// decryptPassword is helper-function that decrypt password with given rsaKey.
func decryptPassword(rsaKey *rsa.PrivateKey, encPwd string) (pwd string, err error) {
	var bsPwd []byte
	if bsPwd, err = base64.StdEncoding.DecodeString(encPwd); err != nil {
		return
	}

	var decPwd []byte
	if decPwd, err = rsa.DecryptOAEP(sha256.New(), rand.Reader, rsaKey, bsPwd, nil); err != nil {
		return
	}
	pwd = string(decPwd)

	return
}

func TestGetSHA256(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GetSHA256))
}

type GetSHA256 struct {
	test []struct {
		msg   string
		value []byte
		want  string
	}
	suite.Suite
}

func (s *GetSHA256) SetupTest() {
	s.test = []struct {
		msg   string
		value []byte
		want  string
	}{
		{
			"must succeed",
			[]byte("MyString"), "bUAyLbdpLICtel7zggMmA47Nh/6GjuV3Id2GdmnaTo4=",
		},
		{
			"must get correct hash on empty string",
			[]byte(""), "kFqmGb8sl4+L4A3w+sctBG33UpA/N3GRkAlLR/1drIM=",
		},
		{
			"must get correct hash on nil slice",
			nil, "kFqmGb8sl4+L4A3w+sctBG33UpA/N3GRkAlLR/1drIM=",
		},
	}
}

func (s *GetSHA256) TestMustGetCorrectHash() {
	for _, t := range s.test {
		// subsequent calls must return same results
		for i := 0; i < 1000; i++ {
			h, err := getSHA256(t.value)
			s.NoError(err, t.msg)
			s.NotEmpty(h, t.msg)
			s.Equal(t.want, h, t.msg)
		}
	}
}

// TestValidateRequestTimestamp is a test function at which we test request timestamp, there could be reasonable skew.
func TestValidateRequestTimestamp(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ValidateRequestTimestamp))
}

type ValidateRequestTimestamp struct{ suite.Suite }

func (s *ValidateRequestTimestamp) TestInTimeframe() {
	s.NoError(validateRequestTimestamp(time.Now().Unix()),
		"must be correct timestamp")
}

func (s *ValidateRequestTimestamp) TestInFuture() {
	s.ErrorIs(validateRequestTimestamp(time.Now().Add(requestTimeframe*2).Unix()), ErrTimeFrame,
		"from far future timestamp")
}

func (s *ValidateRequestTimestamp) TestInPast() {
	s.ErrorIs(validateRequestTimestamp(time.Now().Add(-1*requestTimeframe*2).Unix()), ErrTimeFrame,
		"from far past timestamp")
}

// TestValidateRequestHash is a test at which we validate that idempotency logically correct.
func TestValidateRequestHash(t *testing.T) { suite.Run(t, new(ValidateRequestHash)) }

type ValidateRequestHash struct {
	hash string
	test []struct {
		msg           string
		mockRegRetVal string
		mockRegRetErr error
		wantErr       error
	}
	suite.Suite
}

func (s *ValidateRequestHash) SetupTest() {
	s.hash = "HBSa+/TImW+5PoWeRoVeRwHeLmInGJCeuQeRkm5NMpJW"
	s.test = []struct {
		msg           string
		mockRegRetVal string
		mockRegRetErr error
		wantErr       error
	}{
		// must be idempotent due unique hash
		{
			msg:           "must be idempotent due unique hash",
			mockRegRetVal: "ISwearIRUniqueHash", mockRegRetErr: nil, wantErr: nil,
		},
		{
			msg:           "must be idempotent due empty hash",
			mockRegRetVal: "", mockRegRetErr: nil, wantErr: nil,
		},
		{
			msg:           "must be idempotent due ErrNotExist",
			mockRegRetVal: "whoCaresString", mockRegRetErr: registry.ErrNotExist, wantErr: nil,
		},
		{
			msg:           "must fail idempotency check due same hash",
			mockRegRetVal: s.hash, mockRegRetErr: registry.ErrNotExist, wantErr: ErrIdemp,
		},
		{
			msg:           "must fail idempotency check due unknown error from registry",
			mockRegRetVal: "poweroverwhelming", mockRegRetErr: assert.AnError, wantErr: assert.AnError,
		},
	}
}

func (s *ValidateRequestHash) TestIdempotency() {
	for _, t := range s.test {
		r := new(regKeyHandlerMock)
		r.On("ReadStringProperty", mock.Anything).Return(t.mockRegRetVal, t.mockRegRetErr)
		regKeyHandler = r

		s.ErrorIs(validateRequestHash(s.hash), t.wantErr, t.msg)
	}
}

// TestPasswordsPipeline is test function in which password creation pipeline is tested.
// generate -> assert unique -> encrypt -> decrypt -> compare.
func TestPasswordsPipeline(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GenerateAndEncryptPassword))
}

type GenerateAndEncryptPassword struct{ suite.Suite }

func (s *GenerateAndEncryptPassword) TestPipeline() {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	exponent := base64.StdEncoding.EncodeToString(big.NewInt(int64(rsaKey.PublicKey.E)).Bytes())
	modulus := base64.StdEncoding.EncodeToString(rsaKey.PublicKey.N.Bytes())

	numCycles := 1000
	passwords := make(map[string]int, numCycles)

	for i := 0; i < numCycles; i++ {
		pwd, err := pwdGen.Generate(passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers)
		s.NoError(err)
		s.NotEmpty(pwd)
		s.NotContains(passwords, pwd, "password is unique")

		var encPwd string
		encPwd, err = encryptPassword(modulus, exponent, pwd)
		s.NoError(err)
		s.NotEmpty(encPwd)

		var decPwd string
		decPwd, err = decryptPassword(rsaKey, encPwd)
		s.NoError(err)
		s.Equal(pwd, decPwd)
	}
}

// TestChangeOrCreateUser is a test function at which we cover these cases:
//   * successful password reset
//   * fail password reset
//   * fail user exist check
//   * create user and add to admin group
//   * create user but fail add to admin group
//   * fail to create user
// logic branching:
//   req ->
//     generate password
//     encrypt password
//     check user exist
//       reset password
//       create user
//         add to admins
func TestChangeOrCreateUser(t *testing.T) {
	suite.Run(t, new(ChangeOrCreateUser))
}

type ChangeOrCreateUser struct {
	ctx context.Context
	rsa *rsa.PrivateKey
	usr string
	pwd string
	req request
	suite.Suite
}

const (
	usr = "Kenny"
	pwd = "QWErty123!!!"
)

func (s *ChangeOrCreateUser) SetupSuite() {
	s.ctx = logger.NewContext(context.Background(), zaptest.NewLogger(s.T()))

	var err error
	s.rsa, err = rsa.GenerateKey(rand.Reader, 2048)
	s.NoError(err)

	s.usr = usr
	s.pwd = pwd
	s.req = request{
		Modulus:  base64.StdEncoding.EncodeToString(s.rsa.PublicKey.N.Bytes()),
		Exponent: base64.StdEncoding.EncodeToString(big.NewInt(int64(s.rsa.PublicKey.E)).Bytes()),
		Username: s.usr,
		Expires:  time.Now().Unix(),
		Schema:   "v1",
	}
}

func (s *ChangeOrCreateUser) TestMustResetPassword() {
	u := new(userManagerMock)
	u.On("Exist", s.usr).Return(true, nil)
	u.On("SetPassword", s.usr, s.pwd).Return(nil)
	userManager = u

	g := new(passwordGeneratorMock)
	g.On("Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers).Return(s.pwd, nil)
	pwdGen = g

	encPwd, err := changeOrCreateUser(s.ctx, s.req)
	s.NoError(err)
	s.NotEmpty(encPwd)

	var decPwd string
	decPwd, err = decryptPassword(s.rsa, encPwd)
	s.NoError(err)
	s.Equal(s.pwd, decPwd)
	u.AssertCalled(s.T(), "Exist", s.usr)
	u.AssertCalled(s.T(), "SetPassword", s.usr, s.pwd)
	u.AssertNotCalled(s.T(), "AddToAdministrators")
	g.AssertCalled(s.T(), "Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers)
}

func (s *ChangeOrCreateUser) TestMustFailPasswordReset() { //nolint:dupl
	u := new(userManagerMock)
	u.On("Exist", s.usr).Return(true, nil)
	u.On("SetPassword", s.usr, s.pwd).Return(assert.AnError)
	userManager = u

	g := new(passwordGeneratorMock)
	g.On("Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers).Return(s.pwd, nil)
	pwdGen = g

	encPwd, err := changeOrCreateUser(s.ctx, s.req)
	s.Error(err)

	var decPwd string
	decPwd, err = decryptPassword(s.rsa, encPwd)
	s.NoError(err)
	s.Equal(s.pwd, decPwd)

	u.AssertCalled(s.T(), "Exist", s.usr)
	u.AssertCalled(s.T(), "SetPassword", s.usr, s.pwd)
	u.AssertNotCalled(s.T(), "AddToAdministrators")
	g.AssertCalled(s.T(), "Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers)
}

func (s *ChangeOrCreateUser) TestMustFailUserExistCheck() {
	u := new(userManagerMock)
	u.On("Exist", s.usr).Return(false, assert.AnError)
	userManager = u

	g := new(passwordGeneratorMock)
	g.On("Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers).Return(s.pwd, nil)
	pwdGen = g

	encPwd, err := changeOrCreateUser(s.ctx, s.req)
	s.Error(err)

	var decPwd string
	decPwd, err = decryptPassword(s.rsa, encPwd)
	s.NoError(err)
	s.Equal(s.pwd, decPwd)

	u.AssertCalled(s.T(), "Exist", s.usr)
	u.AssertNotCalled(s.T(), "SetPassword")
	u.AssertNotCalled(s.T(), "AddToAdministrators")
	g.AssertCalled(s.T(), "Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers)
}

func (s *ChangeOrCreateUser) TestMustCreateAdmin() {
	u := new(userManagerMock)
	u.On("Exist", s.usr).Return(false, nil)
	u.On("CreateUser", s.usr, s.pwd).Return(nil)
	u.On("AddToAdministrators", s.usr).Return(nil)
	userManager = u

	g := new(passwordGeneratorMock)
	g.On("Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers).Return(s.pwd, nil)
	pwdGen = g

	encPwd, err := changeOrCreateUser(s.ctx, s.req)
	s.NoError(err)

	var decPwd string
	decPwd, err = decryptPassword(s.rsa, encPwd)
	s.NoError(err)
	s.Equal(s.pwd, decPwd)
	u.AssertCalled(s.T(), "Exist", s.usr)
	u.AssertCalled(s.T(), "CreateUser", s.usr, s.pwd)
	u.AssertCalled(s.T(), "AddToAdministrators", s.usr)
	g.AssertCalled(s.T(), "Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers)
}

func (s *ChangeOrCreateUser) TestMustFailGrantAdmin() {
	u := new(userManagerMock)
	u.On("Exist", s.usr).Return(false, nil)
	u.On("CreateUser", s.usr, s.pwd).Return(nil)
	u.On("AddToAdministrators", s.usr).Return(assert.AnError)
	userManager = u

	g := new(passwordGeneratorMock)
	g.On("Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers).Return(s.pwd, nil)
	pwdGen = g

	encPwd, err := changeOrCreateUser(s.ctx, s.req)
	s.Error(err)

	var decPwd string
	decPwd, err = decryptPassword(s.rsa, encPwd)
	s.NoError(err)
	s.Equal(s.pwd, decPwd)

	u.AssertCalled(s.T(), "Exist", s.usr)
	u.AssertCalled(s.T(), "CreateUser", s.usr, s.pwd)
	u.AssertCalled(s.T(), "AddToAdministrators", s.usr)
	g.AssertCalled(s.T(), "Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers)
}

func (s *ChangeOrCreateUser) TestMustFailCreateUser() { //nolint:dupl
	u := new(userManagerMock)
	u.On("Exist", s.usr).Return(false, nil)
	u.On("CreateUser", s.usr, s.pwd).Return(assert.AnError)
	userManager = u

	g := new(passwordGeneratorMock)
	g.On("Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers).Return(s.pwd, nil)
	pwdGen = g

	encPwd, err := changeOrCreateUser(s.ctx, s.req)
	s.Error(err)

	var decPwd string
	decPwd, err = decryptPassword(s.rsa, encPwd)
	s.NoError(err)
	s.Equal(s.pwd, decPwd)

	u.AssertCalled(s.T(), "Exist", s.usr)
	u.AssertCalled(s.T(), "CreateUser", s.usr, s.pwd)
	u.AssertNotCalled(s.T(), "AddToAdministrators")
	g.AssertCalled(s.T(), "Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers)
}

func (s *ChangeOrCreateUser) TestCancelContext() {
	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	g := new(passwordGeneratorMock)
	pwdGen = g
	u := new(userManagerMock)
	userManager = u

	encPwd, err := changeOrCreateUser(ctx, s.req)
	s.Empty(encPwd)
	s.Error(err)
	u.AssertNotCalled(s.T(), "Exist")
	u.AssertNotCalled(s.T(), "CreateUser")
	u.AssertNotCalled(s.T(), "AddToAdministrators")
	g.AssertNotCalled(s.T(), "Generate")
}

// TestProcessRequest is to test ProcessRequest by passing valid request and decrypt password.
// Also covers positive cases of changeOrCreateUser.
func TestProcessRequest(t *testing.T) {
	suite.Run(t, new(ProcessRequest))
}

type ProcessRequest struct {
	ctx  context.Context
	rsa  *rsa.PrivateKey
	usr  string
	pwd  string
	req  request
	msg  []byte
	hash string
	suite.Suite
}

func (s *ProcessRequest) SetupSuite() { //nolint:dupl
	s.ctx = logger.NewContext(context.Background(), zaptest.NewLogger(s.T()))

	var err error
	s.rsa, err = rsa.GenerateKey(rand.Reader, 2048)
	s.NoError(err)

	s.usr = usr
	s.pwd = pwd
	s.req = request{
		Modulus:  base64.StdEncoding.EncodeToString(s.rsa.PublicKey.N.Bytes()),
		Exponent: base64.StdEncoding.EncodeToString(big.NewInt(int64(s.rsa.PublicKey.E)).Bytes()),
		Username: s.usr,
		Expires:  time.Now().Unix(),
		Schema:   "v1",
	}
	s.msg, err = messages.NewEnvelope().WithType(UserChangeRequestType).Marshal(s.req)
	s.NoError(err)

	s.hash, err = getSHA256(s.req)
	s.NoError(err)
}

func (s *ProcessRequest) TestMustCreateUser() {
	r := new(regKeyHandlerMock)
	r.On("ReadStringProperty", idempotencyPropName).Return("", nil)
	r.On("WriteStringProperty", idempotencyPropName, s.hash).Return(nil)
	regKeyHandler = r

	g := new(passwordGeneratorMock)
	g.On("Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers).Return(s.pwd, nil)
	pwdGen = g

	u := new(userManagerMock)
	u.On("Exist", s.usr).Return(false, nil)
	u.On("CreateUser", s.usr, s.pwd).Return(nil)
	u.On("AddToAdministrators", s.usr).Return(nil)
	userManager = u

	res, err := processRequest(s.ctx, s.msg)
	s.NoError(err)
	s.NotEmpty(res)
	s.True(res.Success)

	var decPwd string
	decPwd, err = decryptPassword(s.rsa, res.EncryptedPassword)
	s.NoError(err)
	s.Equal(s.pwd, decPwd)

	r.AssertCalled(s.T(), "ReadStringProperty", idempotencyPropName)
	r.AssertCalled(s.T(), "WriteStringProperty", idempotencyPropName, s.hash)
	u.AssertNotCalled(s.T(), "Exist")
	u.AssertCalled(s.T(), "CreateUser", s.usr, s.pwd)
	u.AssertCalled(s.T(), "AddToAdministrators", s.usr)
	g.AssertCalled(s.T(), "Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers)
}

func (s *ProcessRequest) TestMustResetUser() {
	r := new(regKeyHandlerMock)
	r.On("ReadStringProperty", idempotencyPropName).Return("", nil)
	r.On("WriteStringProperty", idempotencyPropName, s.hash).Return(nil)
	regKeyHandler = r

	g := new(passwordGeneratorMock)
	g.On("Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers).Return(s.pwd, nil)
	pwdGen = g

	u := new(userManagerMock)
	u.On("Exist", s.usr).Return(true, nil)
	u.On("SetPassword", s.usr, s.pwd).Return(nil)
	userManager = u

	res, err := processRequest(s.ctx, s.msg)
	s.NoError(err)
	s.NotEmpty(res)
	s.True(res.Success)

	var decPwd string
	decPwd, err = decryptPassword(s.rsa, res.EncryptedPassword)
	s.NoError(err)
	s.Equal(s.pwd, decPwd)

	r.AssertCalled(s.T(), "ReadStringProperty", idempotencyPropName)
	r.AssertCalled(s.T(), "WriteStringProperty", idempotencyPropName, s.hash)
	u.AssertCalled(s.T(), "Exist", s.usr)
	u.AssertNotCalled(s.T(), "AddToAdministrators")
	u.AssertCalled(s.T(), "SetPassword", s.usr, s.pwd)
	g.AssertCalled(s.T(), "Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers)
}

func (s *ProcessRequest) TestCancelContext() {
	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	r := new(regKeyHandlerMock)
	regKeyHandler = r
	g := new(passwordGeneratorMock)
	pwdGen = g
	u := new(userManagerMock)
	userManager = u

	encPwd, err := processRequest(ctx, s.msg)
	s.NotEmpty(encPwd)
	s.Equal(encPwd, response{Error: context.Canceled.Error()})
	s.Error(err)
	r.AssertNotCalled(s.T(), "ReadStringProperty")
	r.AssertNotCalled(s.T(), "WriteStringProperty")
	u.AssertNotCalled(s.T(), "Exist")
	u.AssertNotCalled(s.T(), "CreateUser")
	u.AssertNotCalled(s.T(), "AddToAdministrators")
	g.AssertNotCalled(s.T(), "Generate")
}

// TestHandle is a test function at which we e2e test whole handle by forming
// completely valid request and decrypting response that was written to
// our mocked serial port (we know it coz we mock generator).
func TestHandle(t *testing.T) {
	suite.Run(t, new(Handle))
}

type Handle struct {
	ctx  context.Context
	rsa  *rsa.PrivateKey
	usr  string
	pwd  string
	req  request
	msg  []byte
	hash string
	suite.Suite
}

func (s *Handle) SetupSuite() { //nolint:dupl
	s.ctx = logger.NewContext(context.Background(), zaptest.NewLogger(s.T()))

	var err error
	s.rsa, err = rsa.GenerateKey(rand.Reader, 2048)
	s.NoError(err)

	s.usr = usr
	s.pwd = pwd
	s.req = request{
		Modulus:  base64.StdEncoding.EncodeToString(s.rsa.PublicKey.N.Bytes()),
		Exponent: base64.StdEncoding.EncodeToString(big.NewInt(int64(s.rsa.PublicKey.E)).Bytes()),
		Username: s.usr,
		Expires:  time.Now().Unix(),
		Schema:   "v1",
	}
	s.msg, err = messages.NewEnvelope().WithType(UserChangeRequestType).Marshal(s.req)
	s.NoError(err)

	s.hash, err = getSHA256(s.req)
	s.NoError(err)
}

func (s *Handle) TestMustSucceed() {
	g := new(passwordGeneratorMock)
	g.On("Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers).Return(s.pwd, nil)
	pwdGen = g

	r := new(regKeyHandlerMock)
	r.On("ReadStringProperty", idempotencyPropName).Return("", nil)
	r.On("WriteStringProperty", idempotencyPropName, s.hash).Return(nil)
	regKeyHandler = r

	u := new(userManagerMock)
	u.On("Exist", s.usr).Return(false, nil)
	u.On("CreateUser", s.usr, s.pwd).Return(nil)
	u.On("AddToAdministrators", s.usr).Return(nil)
	userManager = u

	p := new(serialPortMock)
	p.On("WriteJSON", mock.Anything).Return(nil)
	serialPort = p

	h := NewUserHandle()
	h.Handle(s.ctx, s.msg)

	r.AssertCalled(s.T(), "ReadStringProperty", idempotencyPropName)
	r.AssertCalled(s.T(), "WriteStringProperty", idempotencyPropName, s.hash)
	u.AssertCalled(s.T(), "CreateUser", s.usr, s.pwd)
	u.AssertCalled(s.T(), "AddToAdministrators", s.usr)
	g.AssertCalled(s.T(), "Generate",
		passwordLength, passwordNumDigits, passwordNumSymbols, passwordNoUppers)

	// we extract response written to mocked serial port, decrypting it and
	// comparing with one we mocked in password generator at last function we mocked
	m, ok := (p.Calls[0].Arguments.Get(0)).(messages.Message)
	s.True(ok)
	s.NotEqual(messages.Message{}, m)

	var res response
	res, ok = m.Payload.(response)
	s.True(ok)
	s.NotEqual(response{}, res)

	decPwd, err := decryptPassword(s.rsa, res.EncryptedPassword)
	s.NoError(err)
	s.Equal(s.pwd, decPwd)
}

func (s *Handle) TestCancelContext() {
	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	r := new(regKeyHandlerMock)
	regKeyHandler = r
	u := new(userManagerMock)
	userManager = u
	g := new(passwordGeneratorMock)
	pwdGen = g
	p := new(serialPortMock)
	serialPort = p

	h := NewUserHandle()
	h.Handle(ctx, s.msg)
	r.AssertNotCalled(s.T(), "ReadStringProperty")
	r.AssertNotCalled(s.T(), "WriteStringProperty")
	u.AssertNotCalled(s.T(), "Exist")
	u.AssertNotCalled(s.T(), "CreateUser")
	u.AssertNotCalled(s.T(), "AddToAdministrators")
	p.AssertNotCalled(s.T(), "WriteJSON")
	p.AssertNotCalled(s.T(), "Write")
	g.AssertNotCalled(s.T(), "Generate")
}
