package kms

import (
	"context"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/mock"
	"reflect"
	"testing"
)

type clientMock struct {
	mock.Mock
}

func (c *clientMock) Decode(keyId string, ciphertext string) ([]byte, error) {
	return []byte("foo"), nil
}

func TestManager_Parse(t *testing.T) {
	type fields struct {
		ctx    context.Context
		fs     afero.Fs
		client KmsClient
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    SecretMetadataMessage
		wantErr bool
	}{
		{
			name: "simple",
			fields: fields{
				ctx:    context.Background(),
				fs:     afero.NewMemMapFs(),
				client: &clientMock{},
			},
			args: args{
				data: []byte(`{"/opt/yaga/secret": {"keyId":"abj12345678901234567","ciphertext":"AAAAAQAAABRhYmoxYjVtMXZycjEyM2htMTQxMAAAABBjwwC+iR9EaNMoiJDspYDQAAAADKuBkwwi34F9IvraEP4JK0w4fMm9bTRo4+j6MP3I4WJ3Il5GlDutCn0I3WRybY4jgXNeQX++QOBT"}}`),
			},
			want: map[string]Secret{
				"/opt/yaga/secret": {
					KeyId:      "abj12345678901234567",
					Ciphertext: "AAAAAQAAABRhYmoxYjVtMXZycjEyM2htMTQxMAAAABBjwwC+iR9EaNMoiJDspYDQAAAADKuBkwwi34F9IvraEP4JK0w4fMm9bTRo4+j6MP3I4WJ3Il5GlDutCn0I3WRybY4jgXNeQX++QOBT",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Manager{
				ctx:    tt.fields.ctx,
				fs:     tt.fields.fs,
				client: tt.fields.client,
			}
			got, err := m.Parse(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() got = %v, want %v", got, tt.want)
			}
		})
	}
}
