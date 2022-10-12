package cm

import (
	"bytes"
	"context"
	"github.com/spf13/afero"
	"marketplace-yaga/linux/internal/persistance"
)

type certifiateClient interface {
	Fetch(certificateId string) ([]byte, error)
}

type Manager struct {
	ctx    context.Context
	fs     afero.Fs
	client certifiateClient
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

type Certificate struct {
	CertificateId string `json:"certificateId"`
}

type CertificateMetadataMessage = map[string]Certificate

func (m *Manager) HandleCertificates(msg CertificateMetadataMessage) ([]string, error) {
	var files []string
	for filepath, cert := range msg {
		certContent, err := m.client.Fetch(cert.CertificateId)
		if err != nil {
			return nil, err
		}

		err = persistance.WriteFile(m.ctx, m.fs, filepath, bytes.NewReader(certContent))
		if err != nil {
			return nil, err
		}
		files = append(files, filepath)
	}
	return files, nil
}
