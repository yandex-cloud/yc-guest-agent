package service

import "errors"

var (
	ErrAlreadyExist = errors.New("already exist")
	ErrNotFound     = errors.New("not found")
	ErrTimeout      = errors.New("timeout")
	ErrDisconnected = errors.New("disconnected")
)
