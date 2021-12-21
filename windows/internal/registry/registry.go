package registry

import (
	"errors"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v4"
	"golang.org/x/sys/windows/registry"
)

const (
	// Registry key security and access rights.
	// See https://msdn.microsoft.com/en-us/library/windows/desktop/ms724878.aspx
	// for details.
	queryValue = 0x00001
	write      = 0x20006
)

const maxRetries = 5

const backoffInterval = 1 * time.Second

type Key struct {
	open KeyOpener
}

type KeyOpener func(access uint32) (StringPropReadWriteCloser, error)

type StringPropReadWriteCloser interface {
	GetStringValue(name string) (val string, valueType uint32, err error)
	SetStringValue(name string, value string) error
	Close() error
}

type StringPropReaderWriter interface {
	ReadStringProperty(name string) (string, error)
	WriteStringProperty(name, value string) error
}

var (
	ErrRelativePathUndef = errors.New("undefined relative path")
	ErrPropertyNameUndef = errors.New("undefined property name")
	ErrNotExist          = syscall.ERROR_FILE_NOT_FOUND
)

// CreateKey creates registry key with retries.
func CreateKey(relativePath string) (existed bool, err error) {
	if relativePath == "" {
		err = ErrRelativePathUndef

		return
	}

	var k registry.Key
	tryCreate := func() (tryErr error) {
		k, existed, tryErr = registry.CreateKey(registry.LOCAL_MACHINE, relativePath, registry.WRITE)

		return tryErr
	}
	defer func() {
		closeErr := k.Close()
		if err == nil {
			err = closeErr
		}
	}()

	err = backoff.Retry(tryCreate,
		backoff.WithMaxRetries(backoff.NewConstantBackOff(backoffInterval), maxRetries))

	return
}

func OpenKey(relativePath string) StringPropReaderWriter {
	return OpenKeyWithOpener(func(access uint32) (reg StringPropReadWriteCloser, err error) {
		var k registry.Key

		tryOpen := func() (tryErr error) {
			k, tryErr = registry.OpenKey(registry.LOCAL_MACHINE, relativePath, access)

			return tryErr
		}

		if err = backoff.Retry(tryOpen,
			backoff.WithMaxRetries(backoff.NewConstantBackOff(backoffInterval),
				maxRetries)); err != nil {
			return nil, err
		}

		return &k, nil
	})
}

func OpenKeyWithOpener(o KeyOpener) StringPropReaderWriter {
	return &Key{
		open: o,
	}
}

func (k *Key) ReadStringProperty(name string) (prop string, err error) {
	if name == "" {
		return "", ErrPropertyNameUndef
	}

	r, err := k.open(queryValue)
	if err != nil {
		return
	}
	defer func() {
		closeErr := r.Close()
		if err == nil {
			err = closeErr
		}
	}()

	// all incorrect types send ErrUnexpectedType
	prop, _, err = r.GetStringValue(name)
	if err != nil {
		return
	}

	return
}

func (k *Key) WriteStringProperty(name, value string) (err error) {
	if name == "" {
		return ErrPropertyNameUndef
	}

	r, err := k.open(write)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := r.Close()
		if err == nil {
			err = closeErr
		}
	}()
	err = r.SetStringValue(name, value)

	return
}
