// need_review
package heartbeat

import (
	"context"
	"fmt"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/pkg/messages"
	"marketplace-yaga/pkg/serial"
	"time"

	"go.uber.org/zap"
)

func NewSerialTicker(ctx context.Context) (*Ticker, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return &Ticker{
		ctx: ctx,
	}, nil
}

type Ticker struct {
	ctx context.Context
}

func (t *Ticker) Wait() {
	<-t.ctx.Done()
}

func (t *Ticker) Start() (err error) {
	if err = t.ctx.Err(); err != nil {
		return
	}

	go t.do()

	return
}

const reportInterval = 60 * time.Second

const MessageType = "heartbeat"

type status struct {
	Status string
}

var okMsg = status{"ok"}

var serialPort = serial.NewBlockingWriter()

func (t *Ticker) do() {
	tr := time.NewTicker(reportInterval)

	for {
		beat(t.ctx)

		select {
		case <-tr.C:
		case <-t.ctx.Done():
			return
		}
	}
}

func beat(ctx context.Context) {
	m := messages.NewEnvelope().WithType(MessageType).Wrap(okMsg)
	err := serialPort.WriteJSON(m)
	logger.InfoCtx(ctx, err, "write heartbeat to serial port",
		zap.String("message", fmt.Sprintf("%+v", m)))
}
