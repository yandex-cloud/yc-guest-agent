package kms

import (
	"context"
	"encoding/base64"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/kms/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"go.uber.org/zap"
	"marketplace-yaga/pkg/logger"
)

type KmsClient interface {
	Decode(keyId string, ciphertext string) ([]byte, error)
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

func (c *YcClient) Decode(keyId string, ciphertext string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	decrypt, err := c.sdk.KMSCrypto().SymmetricCrypto().Decrypt(
		c.ctx,
		&kms.SymmetricDecryptRequest{
			KeyId:      keyId,
			Ciphertext: decoded,
		})
	logger.DebugCtx(c.ctx, err, "decrypted the secret", zap.String("keyId", keyId))
	if err != nil {
		return nil, err
	}
	return decrypt.Plaintext, nil
}
