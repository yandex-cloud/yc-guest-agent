package messages

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gofrs/uuid"
)

var (
	ErrUndef    = errors.New("undefined")
	ErrMistype  = errors.New("mistype")
	ErrNotFound = errors.New("not found")
)

type Envelope struct {
	Timestamp int64
	Type      string
	ID        string
}

// NewEnvelope return blank envelope struct.
// Common use is:
//   NewEnvelope().WithType("log").Wrap(`"this is fine"`)
//
//   {
//     "Timestamp":1621433132,
//     "Type":"log",
//     "ID":"15c07716-c204-4c92-9f58-083c87a5cd5e",
//     "Payload":"this is fine"
//   }
func NewEnvelope() *Envelope {
	return &Envelope{
		Timestamp: time.Now().UTC().Unix(),
		Type:      "Undefined",
		ID:        uuid.Must(uuid.NewV4()).String(),
	}
}

func (e *Envelope) WithTimestamp(t time.Time) *Envelope {
	e.Timestamp = t.UTC().Unix()

	return e
}

func (e *Envelope) WithType(t string) *Envelope {
	e.Type = t

	return e
}

func (e *Envelope) WithID(id fmt.Stringer) *Envelope {
	e.ID = id.String()

	return e
}

func (e *Envelope) Marshal(p interface{}) ([]byte, error) {
	return json.Marshal(e.Wrap(p))
}

func (e *Envelope) Wrap(p interface{}) Message {
	return Message{
		Envelope: *e,
		Payload:  p,
	}
}

type Message struct {
	Envelope
	Payload interface{}
}

// LookupType seeks for type field and compares with provided one.
func LookupType(d []byte, typ string) (err error) {
	if typ == "" {
		return ErrUndef
	}

	if err = checkTypeField(d); err != nil {
		return fmt.Errorf("field '%v' %w", typ, err)
	}

	var t struct{ Type string }
	if err = json.Unmarshal(d, &t); err != nil {
		return
	}
	if t.Type != typ {
		err = ErrMistype
	}

	return
}

type hasField bool

func (f *hasField) UnmarshalJSON(_ []byte) error {
	*f = true

	return nil
}

func checkTypeField(d []byte) (err error) {
	var f struct{ Type hasField }
	if err = json.Unmarshal(d, &f); err != nil {
		return
	}
	if !f.Type {
		err = ErrNotFound
	}

	return
}

func UnmarshalEnvelope(d []byte) (*Envelope, error) {
	if err := checkEnvelopeFields(d); err != nil {
		return nil, err
	}

	var e Envelope
	if err := json.Unmarshal(d, &e); err != nil {
		return nil, err
	}

	return &e, nil
}

func checkEnvelopeFields(d []byte) (err error) {
	var f struct {
		Timestamp hasField
		Type      hasField
		ID        hasField
	}
	if err = json.Unmarshal(d, &f); err != nil {
		return
	}

	if !f.Timestamp {
		err = fmt.Errorf("timestamp field not found %w", ErrNotFound)
	}
	if !f.Type {
		err = fmt.Errorf("type field not found %w", ErrNotFound)
	}
	if !f.ID {
		err = fmt.Errorf("id field not found %w", ErrNotFound)
	}

	return
}

// UnmarshalPayload field into arbitrary provided type v, from message which encoded in d.
func UnmarshalPayload(d []byte, v interface{}) error {
	if err := checkPayloadField(d); err != nil {
		return fmt.Errorf("field 'payload' %w", err)
	}

	var r json.RawMessage
	s := struct {
		Payload interface{}
	}{
		&r,
	}

	if err := json.Unmarshal(d, &s); err != nil {
		return err
	}

	return json.Unmarshal(r, &v)
}

func checkPayloadField(d []byte) (err error) {
	var f struct{ Payload hasField }
	if err = json.Unmarshal(d, &f); err != nil {
		return
	}
	if !f.Payload {
		err = ErrNotFound
	}

	return
}
