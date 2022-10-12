package lockbox

import (
	"context"
	"encoding/json"
	"github.com/spf13/afero"
	"go.uber.org/zap"
	"marketplace-yaga/pkg/logger"
	"os"
	"path"
)

type Manager struct {
	ctx    context.Context
	fs     afero.Fs
	client LockboxClient
}

func New(ctx context.Context) *Manager {
	return newManager(ctx)
}

func newManager(ctx context.Context) *Manager {
	client := NewClient(ctx)

	return &Manager{
		ctx:    ctx,
		fs:     afero.NewOsFs(),
		client: client,
	}
}

type Secret struct {
	SecretId string `json:"secretId"`
	Key      string `json:"key"`
}

type SecretMetadataMessage = map[string]Secret

func (m Manager) Parse(data []byte) (SecretMetadataMessage, error) {
	var msg SecretMetadataMessage
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (m *Manager) HandleSecrets(msg SecretMetadataMessage) ([]string, error) {
	var files []string
	for filepath, secret := range msg {
		err := m.writeFile(filepath, secret)
		if err != nil {
			return nil, err
		}
		files = append(files, filepath)
	}
	return files, nil
}

func (m *Manager) writeFile(filepath string, secret Secret) error {
	logOpts := []zap.Field{
		zap.String("filepath", filepath),
		zap.String("secretId", secret.SecretId),
		zap.String("key", secret.Key),
	}
	plaintext, err := m.client.Fetch(secret.SecretId, secret.Key)
	if err != nil {
		return err
	}
	dir, _ := path.Split(filepath)
	err = m.fs.MkdirAll(dir, 0700)
	logger.DebugCtx(m.ctx, err, "created all folders along file path", logOpts...)
	if err != nil {
		return err
	}

	file, err := m.fs.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0600)
	logger.DebugCtx(m.ctx, err, "opened the file in write mode", logOpts...)
	if err != nil {
		return err
	}

	_, err = file.Write(plaintext)
	logger.DebugCtx(m.ctx, err, "written the secret", logOpts...)
	if err != nil {
		return err
	}

	err = file.Close()
	logger.DebugCtx(m.ctx, err, "closed the file", logOpts...)
	if err != nil {
		return err
	}
	return nil
}
