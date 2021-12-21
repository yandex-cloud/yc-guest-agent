package httpx

import "errors"

var (
	ErrNilCtx        = errors.New("provided nil context")
	ErrEmptyUA       = errors.New("provided empty user agent")
	ErrEmptyEndpoint = errors.New("provided empty endpoint agent")
)
