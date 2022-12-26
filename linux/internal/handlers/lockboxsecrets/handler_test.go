package lockboxsecrets

import (
	"marketplace-yaga/linux/internal/lockbox"
	"reflect"
	"testing"
)

func Test_parse(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    lockbox.SecretMetadataMessage
		wantErr bool
	}{
		{
			name: "simple",
			args: args{
				data: []byte(`{"/opt/yaga/secret": {"secretId":"abj12345678901234567","key":"foo"}}`),
			},
			want: map[string]lockbox.Secret{
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
			got, err := parse(tt.args.data)
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
