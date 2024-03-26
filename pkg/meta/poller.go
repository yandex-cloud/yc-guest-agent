package meta

import (
	"context"
	"errors"
	"fmt"
	"io"
	"marketplace-yaga/pkg/logger"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"
)

// HTTPClient is part of default http.Client, need for poller.
type HTTPClient interface {
	Do(request *http.Request) (resp *http.Response, err error)
}

// Poller get data and updates from metadata.
type Poller struct {
	url        string
	lastETag   string
	HTTPClient HTTPClient
}

// NewPoller creates instance of Poller type.
func NewPoller(url string) *Poller {
	return &Poller{
		url:        url,
		lastETag:   "0",
		HTTPClient: http.DefaultClient,
	}
}

const retryTimeout = 60 * time.Second

const retryMinInterval = 1 * time.Second

func (p *Poller) Get(ctx context.Context) (bs []byte, err error) {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = retryMinInterval
	bo.MaxElapsedTime = retryTimeout

	op := func() error {
		opErr := ctx.Err()
		if opErr != nil {
			logger.ErrorCtx(ctx, opErr, "checked deadline or context cancellation")
			return opErr
		}

		bs, opErr = p.get(ctx)

		return opErr
	}

	err = backoff.Retry(op, bo)

	return
}

var ErrStatusNotOK = errors.New("received non 200 response")

const pollerTimeout = 60 * time.Second

func (p *Poller) get(ctx context.Context) ([]byte, error) {
	ctx = logger.NewContext(ctx, logger.FromContext(ctx).With(
		zap.String("url", p.url),
		zap.String("etag", p.lastETag)))

	req, err := createRequest(ctx, p.url, pollerTimeout, p.lastETag)
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	resp, err = p.HTTPClient.Do(req)
	if err != nil {
		logger.ErrorCtx(ctx, err, "received metadata response")
		return nil, err
	}
	defer closeCtx(ctx, resp.Body)

	if resp.StatusCode != http.StatusOK {
		logger.InfoCtx(ctx, err, "context close has failed", zap.Int("statusCode", resp.StatusCode))

		return nil, fmt.Errorf("%w, code: %v", ErrStatusNotOK, resp.StatusCode)
	}
	p.lastETag = resp.Header.Get("ETag")

	return io.ReadAll(resp.Body)
}

func createRequest(ctx context.Context, url string, timeout time.Duration, lastETag string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet,
		url+"?wait_for_change=true&timeout_sec="+fmt.Sprint(timeout.Seconds())+"&last_etag="+lastETag,
		nil)
	if err != nil {
		logger.ErrorCtx(ctx, err, "create request")
		return nil, err
	}
	req.Header.Add("Metadata-Flavor", "Google")

	return req.WithContext(ctx), nil
}

func closeCtx(ctx context.Context, closer io.Closer) {
	if err := closer.Close(); err != nil {
		logger.InfoCtx(ctx, err, "context close has failed")
	}
}
