package kms

import (
	"bytes"
	"context"
	"github.com/spf13/afero"
	"marketplace-yaga/linux/internal/persistance"
)

type Manager struct {
	ctx    context.Context
	fs     afero.Fs
	client KmsClient
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
	KeyId      string `json:"keyId"`
	Ciphertext string `json:"ciphertext"`
}

type SecretMetadataMessage = map[string]Secret

func (m *Manager) HandleSecrets(msg SecretMetadataMessage) ([]string, error) {
	var files []string
	for filepath, secret := range msg {
		plaintext, err := m.client.Decode(secret.KeyId, secret.Ciphertext)
		if err != nil {
			return nil, err
		}

		err = persistance.WriteFile(m.ctx, m.fs, filepath, bytes.NewReader(plaintext))
		if err != nil {
			return nil, err
		}
		files = append(files, filepath)
	}
	return files, nil
}
