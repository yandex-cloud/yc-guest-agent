package users

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"marketplace-yaga/windows/internal/registry"
	"time"
)

const UserChangeRequestType = "UserChangeRequest"

// request is struct of json passed from metadata.
type request struct {
	Modulus  string
	Exponent string
	Username string
	Expires  int64
	Schema   string
}

// getSHA256 creates base64 string of sha256 hash of provided byte slice.
func getSHA256(v interface{}) (s string, err error) {
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

// validateRequestHash validates request hash.
// We hash every request with sha256 to check if we already processed request.
// That way we protect ourselves from situation in which something will accidentally pass same request again.
func validateRequestHash(reqHash string) error {
	lastRequestHash, err := regKeyHandler.ReadStringProperty(idempotencyPropName)
	// if property does not exist - request is also idempotent
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}

	if lastRequestHash != "" && lastRequestHash == reqHash {
		return ErrIdemp
	}

	return nil
}

// validateRequestTimestamp validates request timestamp.
// We check timestamp, allowing some time skew, must hit [-RequestTimeframe | time.now() | RequestTimeframe+].
func validateRequestTimestamp(expires int64) (err error) {
	e := time.Unix(expires, 0)

	// every request must be fresh
	// also allows some clock skew
	if time.Since(e) > requestTimeframe || requestTimeframe < time.Until(e) {
		err = ErrTimeFrame

		return
	}

	return
}
