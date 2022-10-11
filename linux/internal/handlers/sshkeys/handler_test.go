package sshkeys

import (
	"github.com/stretchr/testify/mock"
)

//goland:noinspection GoUnusedType
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
