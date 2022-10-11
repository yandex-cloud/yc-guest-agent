package meta

import (
	"context"
	"marketplace-yaga/pkg/logger"
	"sync"
	"time"

	"go.uber.org/zap"
)

type pollerGet interface {
	Get(ctx context.Context) ([]byte, error)
}

type MetadataChangeHandler interface {
	Handle(ctx context.Context, data []byte)
	String() string
}

type MetadataWatcher struct {
	ctx          context.Context
	m            sync.Mutex
	timeToHandle time.Duration
}

const handleTimeout = time.Minute

func NewMetadataWatcher(ctx context.Context) *MetadataWatcher {
	return &MetadataWatcher{
		ctx:          ctx,
		timeToHandle: handleTimeout,
	}
}

func (w *MetadataWatcher) AddWatch(url string, handler MetadataChangeHandler) {
	ctx := logger.NewContext(w.ctx, logger.FromContext(w.ctx).With(zap.Stringer("event", handler)))

	logger.InfoCtx(ctx, nil, "start metadata watch")
	poller := NewPoller(url)

	go w.watch(ctx, poller, handler)
}

// Wait until watcher stop.
func (w *MetadataWatcher) Wait() {
	<-w.ctx.Done()
}

func (w *MetadataWatcher) watch(ctx context.Context, p pollerGet, h MetadataChangeHandler) {
	for {
		err := ctx.Err()
		logger.DebugCtx(ctx, ctx.Err(), "checked deadline or context cancellation")
		if err != nil {
			return
		}

		var data []byte
		data, err = p.Get(ctx)
		logger.DebugCtx(ctx, err, "got new metadata", zap.ByteString("content", data))
		if err != nil {
			continue
		}

		w.syncCall(func() {
			handleCtx, handleCtxCancel := context.WithTimeout(ctx, w.timeToHandle)
			h.Handle(handleCtx, data)
			handleCtxCancel()
		})
	}
}

func (w *MetadataWatcher) syncCall(f func()) {
	w.m.Lock()
	defer w.m.Unlock()

	f()
}
