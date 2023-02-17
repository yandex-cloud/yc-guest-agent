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

const portName = "COM4"

const portBaud = 115200

const maxRetries = 10

func Init() (err error) {
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

type blockingPort struct {
	wl sync.Mutex
}

func (p *blockingPort) Close() error {
	if port == nil {
		return ErrNotInitialized
	}

	return port.Close()
}

func (p *blockingPort) Write(bs []byte) (int, error) {
	if port == nil {
		return 0, ErrNotInitialized
	}

	p.wl.Lock()
	defer p.wl.Unlock()

	return port.Write(bs)
}

func (p *blockingPort) WriteJSON(j interface{}) error {
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
	return new(blockingPort)
}
