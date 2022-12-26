package users

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"github.com/spf13/afero"
	"io"
	"io/fs"
	"time"
)

//goland:noinspection GoUnusedConst
const UserChangeRequestType = "UserChangeRequest"

// request is struct of json passed from metadata.
type request struct {
	Modulus  string
	Exponent string
	Username string
	Expires  int64
	Schema   string
}

type RequestManager struct {
	fs afero.Fs
}

func NewRequestManager(fs afero.Fs) RequestManager {
	return RequestManager{
		fs: fs,
	}
}

// GetSHA256 creates base64 string of sha256 hash of provided byte slice.
func (r *RequestManager) GetSHA256(v interface{}) (s string, err error) {
	buf := bytes.Buffer{}
	if err = gob.NewEncoder(&buf).Encode(v); err != nil {
		return
	}

	h := sha256.New()
	_, err = h.Write(buf.Bytes())
	if err != nil {
		return
	}

	b := h.Sum(nil)
	s = base64.StdEncoding.EncodeToString(b)

	return
}

// ValidateRequestHash validates request hash.
// We hash every request with sha256 to check if we already processed request.
// That way we protect ourselves from situation in which something will accidentally pass same request again.
func (r *RequestManager) ValidateRequestHash(reqHash string) error {
	file, err := r.fs.Open(idempotencyFile)
	// if property does not exist - request is also idempotent
	if err != nil {
		switch err.(type) {
		case *fs.PathError:
			return nil
		default:
			return err
		}
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	lastRequestHash := string(content)
	if lastRequestHash != "" && lastRequestHash == reqHash {
		return ErrIdemp
	}

	return nil
}

// ValidateRequestTimestamp validates request timestamp.
// We check timestamp, allowing some time skew, must hit [-RequestTimeframe | time.now() | RequestTimeframe+].
func (r *RequestManager) ValidateRequestTimestamp(expires int64) (err error) {
	e := time.Unix(expires, 0)

	// every request must be fresh
	// also allows some clock skew
	if time.Since(e) > requestTimeframe || requestTimeframe < time.Until(e) {
		err = ErrTimeFrame

		return
	}

	return
}
