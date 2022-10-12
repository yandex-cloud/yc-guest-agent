package lockbox

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

func (c *clientMock) Fetch(secretId string, key string) ([]byte, error) {
	return []byte("foo"), nil
}

func TestManager_Fetch(t *testing.T) {
	type fields struct {
		ctx    context.Context
		fs     afero.Fs
		client LockboxClient
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
				data: []byte(`{"/opt/yaga/secret": {"secretId":"abj12345678901234567","key":"foo"}}`),
			},
			want: map[string]Secret{
				"/opt/yaga/secret": {
					SecretId: "abj12345678901234567",
					Key:      "foo",
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
