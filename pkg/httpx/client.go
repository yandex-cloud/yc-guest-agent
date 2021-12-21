package httpx

import (
	"context"
	"fmt"
	"io"
	"marketplace-yaga/pkg/logger"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gofrs/uuid"
	"go.uber.org/zap"
)

type Client struct {
	ctx context.Context
	r   *resty.Client
}

const (
	retries      = 5
	retryWait    = 1 * time.Second
	retryMaxWait = 30 * time.Second
	userAgentKey = "User-Agent"
	requestIDKey = "X-Request-ID"
)

func New(ctx context.Context, endpoint string, ua string) (*Client, error) {
	if ctx == nil {
		return nil, ErrNilCtx
	}

	if endpoint == "" {
		return nil, ErrEmptyEndpoint
	}

	if ua == "" {
		return nil, ErrEmptyUA
	}

	r := resty.New().
		SetHostURL(endpoint).
		SetRetryCount(retries).
		SetRetryWaitTime(retryWait).
		SetRetryMaxWaitTime(retryMaxWait).
		SetHeader(userAgentKey, ua).
		SetRedirectPolicy(resty.NoRedirectPolicy()).
		OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
			requestID, err := uuid.NewV4()
			if err != nil {
				return err
			}
			req.SetHeader(requestIDKey, requestID.String())

			return nil
		}).
		OnAfterResponse(func(c *resty.Client, r *resty.Response) error {
			logger.DebugCtx(ctx, nil, "request",
				zap.String("request-id", r.Request.Header.Get(requestIDKey)),
				zap.String("received", r.ReceivedAt().String()),
				zap.String("time", r.Time().String()),
				zap.String("status", r.Status()),
				zap.String("proto", r.Proto()),
				zap.String("method", r.Request.Method),
				zap.String("url", r.Request.URL))

			return nil
		})

	return &Client{ctx: ctx, r: r}, nil
}

func (c *Client) R() *resty.Request {
	return c.r.R()
}

func (c *Client) Download(filepath, url string) error {
	r, err := c.R().SetOutput(filepath).Get(url)
	logger.DebugCtx(c.ctx, err, "download", zap.String("url", url), zap.String("filepath", filepath))
	if err != nil {
		return err
	}
	if r.IsError() {
		return fmt.Errorf("download failed with code: %v", r.StatusCode())
	}

	return nil
}

func (c *Client) Downloader(w io.Writer, url string) error {
	r, err := c.R().SetDoNotParseResponse(true).Get(url)
	logger.DebugCtx(c.ctx, err, "download",
		zap.String("url", url))
	if err != nil {
		return err
	}
	if r.IsError() {
		return fmt.Errorf("download failed with code: %v, body: %v", r.StatusCode(), string(r.Body()))
	}

	_, err = io.Copy(w, r.RawBody())
	if err != nil {
		return err
	}

	return r.RawBody().Close()
}

func (c *Client) GetClient() *http.Client {
	return c.r.GetClient()
}
