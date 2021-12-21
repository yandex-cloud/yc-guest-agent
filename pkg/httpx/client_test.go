package httpx

import (
	"context"
	"errors"
	"fmt"
	"marketplace-yaga/pkg/logger"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
)

func TestNewClient(t *testing.T) {
	suite.Run(t, new(newClientTests))
}

type newClientTests struct{ suite.Suite }

// TestNew - checks client creation.
func (s *newClientTests) TestNew() {
	ctx := logger.NewContext(context.Background(), zaptest.NewLogger(s.T()))
	tests := []struct {
		gotCtx      context.Context
		gotEndpoint string
		gotUa       string
		expectErr   error
	}{
		{nil, "endpoint", "ua", ErrNilCtx},
		{ctx, "", "ua", ErrEmptyEndpoint},
		{ctx, "endpoint", "", ErrEmptyUA},
	}

	for _, t := range tests {
		_, err := New(t.gotCtx, t.gotEndpoint, t.gotUa)
		s.ErrorIs(err, t.expectErr)
	}
}

func TestClient(t *testing.T) {
	suite.Run(t, new(clientTests))
}

type clientTests struct {
	h *httptest.Server
	c *Client

	suite.Suite
}

const userAgent = "my.test.ua"
const testFilecontent = `radio free zerg`

func (s *clientTests) SetupSuite() {
	s.h = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, testFilecontent)
	}))

	s.c, _ = New(
		logger.NewContext(context.Background(), zaptest.NewLogger(s.T())),
		s.h.URL,
		userAgent)
}

func (s *clientTests) TearDownSuite() {
	s.h.Close()
}

const testURL = "/anything"

// TestUserAgent - checks if user-agent is set.
func (s *clientTests) TestUserAgent() {
	resp, err := s.c.R().Get(testURL)
	s.NoError(err)

	ua := resp.Request.Header.Get(userAgentKey)
	s.Equal(userAgent, ua)
}

// TestRequestID - checks if request-id is set.
func (s *clientTests) TestRequestID() {
	resp, err := s.c.R().Get(testURL)
	s.NoError(err)

	reqID := resp.Request.Header.Get(requestIDKey)
	s.NotEmpty(reqID)
}

// TestRequestIDUnique - checks if request-id is unique between requests.
func (s *clientTests) TestRequestIDUnique() {
	resp1, err := s.c.R().Get(testURL)
	s.NoError(err)

	resp2, err := s.c.R().Get(testURL)
	s.NoError(err)

	reqID1 := resp1.Request.Header.Get(requestIDKey)
	reqID2 := resp2.Request.Header.Get(requestIDKey)

	s.NotEqual(reqID1, reqID2)
}

// TestRequestIDisUUID4 - checks if request-id is valid uuid.
func (s *clientTests) TestRequestIDisUUID4() {
	resp, err := s.c.R().Get(testURL)
	s.NoError(err)

	reqID := resp.Request.Header.Get(requestIDKey)
	s.NotEmpty(reqID)

	_, err = uuid.FromString(reqID)
	s.NoError(err)
}

// TestDownload - checks that func could download and save file.
func (s *clientTests) TestDownload() {
	testFilepath, err := getTempFilepath()
	s.Require().NoError(err)
	s.Require().NotEmpty(testFilepath)

	s.NoError(s.c.Download(testFilepath, testURL))

	got, err := os.ReadFile(testFilepath)
	s.NoError(err)
	s.Equal(testFilecontent, string(got))

	_ = os.Remove(testFilepath)
}

func (s *clientTests) TestDownloader() {
	testFilepath, err := getTempFilepath()
	s.Require().NoError(err)
	s.Require().NotEmpty(testFilepath)

	f, err := os.Create(testFilepath)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	s.NoError(s.c.Downloader(f, testURL))

	got, err := os.ReadFile(testFilepath)
	s.NoError(err)
	s.Equal(testFilecontent, string(got))

	_ = os.Remove(testFilepath)
}

func getTempFilepath() (string, error) {
	timeout := 1 * time.Second
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("failed to generate filepath in %v seconds", timeout)
		}

		u, err := uuid.NewV4()
		if err != nil {
			return "", err
		}
		f := filepath.Join(os.TempDir(), u.String())

		_, err = os.Stat(f)
		if errors.Is(err, os.ErrNotExist) {
			return f, nil
		}
	}
}
