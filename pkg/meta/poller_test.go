package meta

import (
	"context"
	"marketplace-yaga/pkg/logger"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestPoller_Get(t *testing.T) {
	at := assert.New(t)
	l := zaptest.NewLogger(t)
	ctx, ctxCancel := context.WithCancel(logger.NewContext(context.Background(), l))
	defer ctxCancel()

	var testCtx context.Context
	var textCtxCancel context.CancelFunc
	var mux http.ServeMux
	var server *httptest.Server
	var poller *Poller
	var data []byte
	var err error

	// test ok
	testCtx, textCtxCancel = context.WithCancel(ctx)
	mux = http.ServeMux{}
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		at.EqualValues(request.URL.Path, "/asd")
		writer.Header().Set("ETag", "123")
		_, _ = writer.Write([]byte("OK"))
	})
	server = httptest.NewServer(&mux)

	poller = NewPoller(server.URL + "/asd")
	data, err = poller.Get(testCtx)
	at.Equal(data, []byte("OK"))
	at.NoError(err)

	server.Close()
	textCtxCancel()

	// test etag
	testCtx, textCtxCancel = context.WithCancel(ctx)
	mux = http.ServeMux{}
	var callTimes int
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		at.EqualValues(request.URL.Path, "/asd")

		callTimes++
		if callTimes > 1 {
			at.Equal("123", request.Header.Get("ETag"))
		}

		writer.Header().Set("ETag", "123")
		_, _ = writer.Write([]byte("OK"))
	})
	server = httptest.NewServer(&mux)

	poller = NewPoller(server.URL + "/asd")
	data, err = poller.Get(testCtx)
	at.Equal([]byte("OK"), data)
	at.NoError(err)

	server.Close()
	textCtxCancel()
}
