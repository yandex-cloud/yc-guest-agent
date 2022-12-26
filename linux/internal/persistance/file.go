package persistance

import (
	"context"
	"fmt"
	"github.com/spf13/afero"
	"go.uber.org/zap"
	"io"
	"marketplace-yaga/pkg/logger"
	"os"
	"path"
)

func WriteFile(ctx context.Context, fs afero.Fs, filepath string, content io.Reader) error {
	logOpts := []zap.Field{
		zap.String("filepath", filepath),
	}
	dir, _ := path.Split(filepath)
	err := fs.MkdirAll(dir, 0700)
	logger.DebugCtx(ctx, err, "created all folders along file path", logOpts...)
	if err != nil {
		return err
	}

	file, err := fs.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	logger.DebugCtx(ctx, err, "opened the file in write mode", logOpts...)
	if err != nil {
		return err
	}

	n, err := io.Copy(file, content)
	logger.DebugCtx(ctx, err, fmt.Sprintf("%d bytes written to file", n), logOpts...)
	if err != nil {
		return err
	}

	err = file.Close()
	logger.DebugCtx(ctx, err, "closed the file", logOpts...)
	if err != nil {
		return err
	}
	return nil
}
