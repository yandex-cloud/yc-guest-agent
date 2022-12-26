package cm

import (
	"context"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/certificatemanager/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"go.uber.org/zap"
	"marketplace-yaga/pkg/logger"
)

type Certificate struct {
	// ID of the certificate.
	CertificateId string `json:"certificateId,omitempty"`
	// PEM-encoded certificate chain content of the certificate.
	CertificateChain []string `json:"certificateChain,omitempty"`
	// PEM-encoded private key content of the certificate.
	PrivateKey string `json:"privateKey,omitempty"`
}

type YcClient struct {
	ctx context.Context
	sdk *ycsdk.SDK
}

func NewClient(ctx context.Context) *YcClient {
	sdk, err := ycsdk.Build(ctx, ycsdk.Config{
		Credentials: ycsdk.InstanceServiceAccount(),
	})
	if err != nil {
		logger.InfoCtx(ctx, err, "can not create SDK to decrypt secrets")
	}
	return &YcClient{
		ctx: ctx,
		sdk: sdk,
	}
}

func (c *YcClient) Fetch(certificateId string) (*Certificate, error) {
	cert, err := c.sdk.CertificatesData().CertificateContent().Get(
		c.ctx,
		&certificatemanager.GetCertificateContentRequest{
			CertificateId:    certificateId,
			PrivateKeyFormat: certificatemanager.PrivateKeyFormat_PKCS8,
		})
	logger.DebugCtx(c.ctx, err, "failed to fetch the secret",
		zap.String("certificateId", certificateId),
	)
	if err != nil {
		return nil, err
	}

	return &Certificate{
		CertificateId:    cert.CertificateId,
		CertificateChain: cert.CertificateChain,
		PrivateKey:       cert.PrivateKey,
	}, nil
}
