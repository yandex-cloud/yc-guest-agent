package lockbox

import (
	"context"
	"fmt"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/lockbox/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"go.uber.org/zap"
	"marketplace-yaga/pkg/logger"
)

type LockboxClient interface {
	Fetch(secretId string, key string) ([]byte, error)
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

func (c *YcClient) Fetch(secretId string, key string) ([]byte, error) {
	s, err := c.sdk.LockboxPayload().Payload().Get(
		c.ctx,
		&lockbox.GetPayloadRequest{
			SecretId: secretId,
		})
	if err != nil {
		logger.DebugCtx(c.ctx, err, "failed to fetch the secret",
			zap.String("secretId", secretId),
		)
		return nil, err
	}

	logger.DebugCtx(c.ctx, err, "fetched the secret",
		zap.String("secretId", secretId),
		zap.String("versionId", s.VersionId),
	)
	for _, e := range s.Entries {
		if e.Key == key {
			switch v := e.Value.(type) {
			case *lockbox.Payload_Entry_BinaryValue:
				return v.BinaryValue, nil
			case *lockbox.Payload_Entry_TextValue:
				return []byte(v.TextValue), nil
			case nil:
				return nil, fmt.Errorf(`nil value of the secret`)
			default:
				return nil, fmt.Errorf(`unexpected value type of the secret`)
			}
		}
	}
	return nil, fmt.Errorf(`provided key %s was not found in the secret`, key)
}
