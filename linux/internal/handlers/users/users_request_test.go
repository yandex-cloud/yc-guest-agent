package users

import (
	"errors"
	"github.com/spf13/afero"
	"testing"
	"time"
)

func TestRequestManager_GetSHA256(t *testing.T) {
	type fields struct {
		fs afero.Fs
	}
	type args struct {
		v interface{}
	}

	type foo struct {
		Bar string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantS   string
		wantErr bool
	}{
		{
			name:   "Foo#Bar",
			fields: fields{fs: afero.NewMemMapFs()},
			args: args{
				foo{
					Bar: "1",
				},
			},
			wantS:   "z2HBloVUkzzoqMKocjHYynPIBGaviNaYSRzheuSRwss=",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RequestManager{
				fs: tt.fields.fs,
			}
			gotS, err := r.GetSHA256(tt.args.v)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSHA256() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotS != tt.wantS {
				t.Errorf("GetSHA256() gotS = %v, want %v", gotS, tt.wantS)
			}
		})
	}
}

func TestRequestManager_ValidateRequestHash(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		r := &RequestManager{
			fs: afero.NewMemMapFs(),
		}

		f, _ := r.fs.Create(idempotencyFile)
		_, _ = f.Write([]byte("456"))
		if err := r.ValidateRequestHash("123"); err != nil {
			t.Errorf("ValidateRequestHash() error = %v", err)
		}
	})

	t.Run("no file", func(t *testing.T) {
		r := &RequestManager{
			fs: afero.NewMemMapFs(),
		}

		if err := r.ValidateRequestHash("123"); err != nil {
			t.Errorf("ValidateRequestHash() error = %v error", err)
		}
	})

	t.Run("old hash", func(t *testing.T) {
		r := &RequestManager{
			fs: afero.NewMemMapFs(),
		}
		f, _ := r.fs.Create(idempotencyFile)
		_, _ = f.Write([]byte("123"))

		if err := r.ValidateRequestHash("123"); !errors.Is(err, ErrIdemp) {
			t.Errorf("ValidateRequestHash() error = %v error wande", err)
		}
	})

}

func TestRequestManager_ValidateRequestTimestamp(t *testing.T) {
	type fields struct {
		fs afero.Fs
	}
	type args struct {
		expires int64
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			fields: fields{
				fs: afero.NewMemMapFs(),
			},
			args: args{
				expires: time.Now().Unix(),
			},
			wantErr: false,
		},
		{
			name: "old",
			fields: fields{
				fs: afero.NewMemMapFs(),
			},
			args: args{
				expires: time.Now().Add(-6 * time.Minute).Unix(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RequestManager{
				fs: tt.fields.fs,
			}
			if err := r.ValidateRequestTimestamp(tt.args.expires); (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequestTimestamp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
