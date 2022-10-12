package persistance

import (
	"context"
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

	file, err := fs.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0600)
	logger.DebugCtx(ctx, err, "opened the file in write mode", logOpts...)
	if err != nil {
		return err
	}

	var data []byte
	_, err = content.Read(data)
	if err != nil {
		return err
	}

	_, err = file.Write(data)
	logger.DebugCtx(ctx, err, "written the content", logOpts...)
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
