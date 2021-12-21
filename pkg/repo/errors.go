package repo

import "errors"

var (
	ErrNilCtx        = errors.New("provided nil context")
	ErrNilFs         = errors.New("provided nil fs interface")
	ErrEmptyRoot     = errors.New("provided empty root")
	ErrEmptyFilename = errors.New("provided empty filename")
	ErrNotFound      = errors.New("not found")
	ErrNotDir        = errors.New("not a directory")
	ErrNotFile       = errors.New("not a file")
	ErrAlreadyAdded  = errors.New("already added")
)
