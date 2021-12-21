package mock

import (
	"marketplace-yaga/pkg/passwords"
)

type MockGenerator struct {
	myPassword string
	myError    error
}

func NewMockGenerator(password string, err error) passwords.GeneratorInterface {
	return &MockGenerator{
		myPassword: password,
		myError:    err,
	}
}

func (m *MockGenerator) Generate(length, numDigits, numSymbols uint, noUpper bool) (string, error) {
	if length < numDigits+numSymbols {
		return "", passwords.ErrLengthTooShort
	}

	if m.myError != nil {
		return "", m.myError
	}

	return m.myPassword, nil
}
