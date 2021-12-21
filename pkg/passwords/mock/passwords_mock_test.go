package mock

import (
	"errors"
	"marketplace-yaga/pkg/passwords"
	"testing"
)

func TestMockGenerator(t *testing.T) {
	expectedError := errors.New("my random error")
	expectedPassword := "mySuperSecretPassword"

	t.Run("catch errLengthTooShort", func(t *testing.T) {
		m := NewMockGenerator(expectedPassword, expectedError)

		_, err := m.Generate(15, 16, 17, false)
		if !errors.Is(err, passwords.ErrLengthTooShort) {
			t.Error(err)
		}

		_, err = m.Generate(0, 16, 17, false)
		if !errors.Is(err, passwords.ErrLengthTooShort) {
			t.Error(err)
		}

		_, err = m.Generate(0, 0, 17, false)
		if !errors.Is(err, passwords.ErrLengthTooShort) {
			t.Error(err)
		}

		_, err = m.Generate(0, 16, 0, false)
		if !errors.Is(err, passwords.ErrLengthTooShort) {
			t.Error(err)
		}
	})

	t.Run("should return error", func(t *testing.T) {
		m := NewMockGenerator(expectedPassword, expectedError)
		_, err := m.Generate(3, 2, 1, false)
		if !errors.Is(err, expectedError) {
			t.Error(err)
		}

		m = NewMockGenerator("", expectedError)
		_, err = m.Generate(3, 2, 1, false)
		if !errors.Is(err, expectedError) {
			t.Error(err)
		}
	})

	t.Run("should return password", func(t *testing.T) {
		m := NewMockGenerator("mySuperSecretPassword", nil)
		pwd, err := m.Generate(3, 2, 1, false)
		if err != nil {
			t.Error(err)
		}

		if pwd != expectedPassword {
			t.Errorf("expected password: %v, got: %v ", expectedPassword, pwd)
		}
	})
}
