package serial

import (
	"encoding/json"
	"errors"
	"io"
	"sync"

	"github.com/cenkalti/backoff/v4"
	"github.com/tarm/serial"
)

var once sync.Once

var port io.WriteCloser

var writeLock sync.Mutex

const portBaud = 115200

const maxRetries = 10

func Init(portName string) (err error) {
	once.Do(func() {
		var p io.WriteCloser
		tryOpen := func() (tryErr error) {
			p, tryErr = serial.OpenPort(&serial.Config{Name: portName, Baud: portBaud})

			return
		}
		var b backoff.ConstantBackOff

		if err = backoff.Retry(tryOpen, backoff.WithMaxRetries(&b, maxRetries)); err == nil {
			port = p
		}
	})

	return
}

var ErrNotInitialized = errors.New("accessed methods of uninitialized port")

type serialWriter struct{}

func (p *serialWriter) Close() error {
	if port == nil {
		return ErrNotInitialized
	}

	return port.Close()
}

func (p *serialWriter) Write(bs []byte) (int, error) {
	if port == nil {
		return 0, ErrNotInitialized
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	written := 0
	for written < len(bs) {
		n, err := port.Write(bs[written:])
		written += n
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, io.ErrShortWrite
		}
	}

	return written, nil
}

func (p *serialWriter) WriteJSON(j interface{}) error {
	if port == nil {
		return ErrNotInitialized
	}

	bs, err := json.Marshal(j)
	if err != nil {
		return err
	}
	_, err = p.Write(append(bs, []byte("\n")...))

	return err
}

type BlockingWriter interface {
	io.WriteCloser
	WriteJSON(j interface{}) error
}

func NewBlockingWriter() BlockingWriter {
	return new(serialWriter)
}
