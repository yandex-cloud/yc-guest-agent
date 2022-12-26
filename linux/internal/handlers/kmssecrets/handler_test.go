package kmssecrets

import (
	"marketplace-yaga/linux/internal/kms"
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
		want    kms.SecretMetadataMessage
		wantErr bool
	}{
		{
			name: "simple",
			args: args{
				data: []byte(`{"/opt/yaga/secret": {"keyId":"abj12345678901234567","ciphertext":"AAAAAQAAABRhYmoxYjVtMXZycjEyM2htMTQxMAAAABBjwwC+iR9EaNMoiJDspYDQAAAADKuBkwwi34F9IvraEP4JK0w4fMm9bTRo4+j6MP3I4WJ3Il5GlDutCn0I3WRybY4jgXNeQX++QOBT"}}`),
			},
			want: map[string]kms.Secret{
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
