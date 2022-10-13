package cm

import (
	"bytes"
	"context"
	"github.com/spf13/afero"
	"marketplace-yaga/linux/internal/persistance"
)

type certificateClient interface {
	Fetch(certificateId string) (*Certificate, error)
}

type Manager struct {
	ctx    context.Context
	fs     afero.Fs
	client certificateClient
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

type CertificateSpec struct {
	CertificateId string `json:"certificateId"`
}

type CertificateMetadataMessage = map[string]CertificateSpec

func (m *Manager) HandleCertificates(msg CertificateMetadataMessage) ([]string, error) {
	var files []string
	for filepath, spec := range msg {
		cert, err := m.client.Fetch(spec.CertificateId)
		if err != nil {
			return nil, err
		}

		var certContent []byte
		for _, cc := range cert.CertificateChain {
			certContent = append(certContent, []byte(cc)...)
		}
		certContent = append(certContent, []byte(cert.PrivateKey)...)

		err = persistance.WriteFile(m.ctx, m.fs, filepath, bytes.NewReader(certContent))
		if err != nil {
			return nil, err
		}
		files = append(files, filepath)
	}
	return files, nil
}
