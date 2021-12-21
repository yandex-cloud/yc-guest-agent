package meta

import (
	"context"
	"errors"
	"marketplace-yaga/pkg/logger"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"
)

type pollerMock struct {
	mock.Mock
	GetCalled func()
}

func (p *pollerMock) Get(ctx context.Context) ([]byte, error) {
	args := p.Called(ctx)
	if p.GetCalled != nil {
		p.GetCalled()
	}

	return args.Get(0).([]byte), args.Error(1)
}

//var _ pollerGet = &pollerMock{}

type handlerMock struct {
	mock.Mock
	HandleCalled func()
}

func (h *handlerMock) Handle(ctx context.Context, data []byte) {
	_ = h.Called(ctx, data)
	if h.HandleCalled != nil {
		h.HandleCalled()
	}
}

func (h *handlerMock) String() string {
	return h.Called().String(0)
}

//var _ MetadataChangeHandler = &handlerMock{}

func TestEventWatcher_syncCall(t *testing.T) {
	ctx, ctxCancel := context.WithCancel(logger.NewContext(context.Background(), zaptest.NewLogger(t)))
	defer ctxCancel()

	watcher := NewMetadataWatcher(ctx)
	var sum int
	const dest = 100000

	var wg sync.WaitGroup
	wg.Add(dest)

	for i := 0; i < dest; i++ {
		go watcher.syncCall(func() {
			sum++
			wg.Done()
		})
	}

	wg.Wait()

	if sum != dest {
		t.Error(sum)
	}
}

func TestEventWatcher_watch(t *testing.T) {
	ctx, ctxCancel := context.WithCancel(logger.NewContext(context.Background(), zaptest.NewLogger(t)))
	defer ctxCancel()

	var poller *pollerMock
	var handler *handlerMock

	metaBytes := []byte("asdf")

	// test OK, once
	watchCtx, watchCtxCancel := context.WithCancel(ctx)

	poller = &pollerMock{}
	poller.On("Get", watchCtx).Return(metaBytes, nil)

	handler = &handlerMock{}
	handler.On("String").Return("handler mock")
	handler.On("Handle", mock.MatchedBy(func(ctx context.Context) bool {
		if deadline, ok := ctx.Deadline(); ok {
			return deadline.After(time.Now())
		}

		return false
	}), metaBytes).Return()

	watcher := NewMetadataWatcher(watchCtx)
	watcher.timeToHandle = time.Millisecond
	handler.HandleCalled = func() {
		watchCtxCancel()
	}

	watcher.watch(watchCtx, poller, handler)

	poller.AssertNumberOfCalls(t, "Get", 1)
	handler.AssertNumberOfCalls(t, "Handle", 1)

	// test OK twice
	watchCtx, watchCtxCancel = context.WithCancel(ctx)

	poller = &pollerMock{}
	poller.On("Get", watchCtx).Return(metaBytes, nil)

	handler = &handlerMock{}
	handler.On("String").Return("handler mock")
	handler.On("Handle", mock.MatchedBy(func(ctx context.Context) bool {
		if deadline, ok := ctx.Deadline(); ok {
			return deadline.After(time.Now())
		}

		return false
	}), metaBytes).Return()

	watcher = NewMetadataWatcher(watchCtx)
	watcher.timeToHandle = time.Millisecond
	var handleCnt int
	handler.HandleCalled = func() {
		handleCnt++
		if handleCnt == 2 {
			watchCtxCancel()
		}
	}

	watcher.watch(watchCtx, poller, handler)

	poller.AssertNumberOfCalls(t, "Get", 2)
	handler.AssertNumberOfCalls(t, "Handle", 2)

	// test ok after error
	watchCtx, watchCtxCancel = context.WithCancel(ctx)

	poller = &pollerMock{}
	poller.On("Get", watchCtx).Return([]byte(nil), errors.New("test error"))
	poller.GetCalled = func() {
		poller.ExpectedCalls[0].ReturnArguments = []interface{}{metaBytes, nil}
	}

	handler = &handlerMock{}
	handler.On("String").Return("handler mock")
	handler.On("Handle", mock.MatchedBy(func(ctx context.Context) bool {
		if deadline, ok := ctx.Deadline(); ok {
			return deadline.After(time.Now())
		}

		return false
	}), metaBytes).Return()

	watcher = NewMetadataWatcher(watchCtx)
	watcher.timeToHandle = time.Millisecond
	handler.HandleCalled = func() {
		watchCtxCancel()
	}

	watcher.watch(watchCtx, poller, handler)

	poller.AssertNumberOfCalls(t, "Get", 2)
	handler.AssertNumberOfCalls(t, "Handle", 1)
}
