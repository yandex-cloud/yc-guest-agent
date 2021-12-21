package messages

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestSuite(t *testing.T) {
	suite.Run(t, new(MessagesTestSuite))
}

type MessagesTestSuite struct {
	suite.Suite
}

func (suite *MessagesTestSuite) TestLookupType() {
	suite.NoError(LookupType([]byte(`{"type": "UserChangeResponse"}`), "UserChangeResponse"))
	suite.ErrorIs(LookupType([]byte(`{"type": "UserChangeResponse"}`), "Undefined"), ErrMistype)
	suite.ErrorIs(LookupType([]byte(`{"type": "UserChangeResponse"}`), ""), ErrUndef)
	suite.ErrorIs(LookupType([]byte(`{"id":"my_id"}`), "UserChangeResponse"), ErrNotFound)
}

func (suite *MessagesTestSuite) TestUnmarshalPayload() {
	type test struct {
		FieldOne string
		FieldTwo string
	}
	var v test

	suite.NoError(UnmarshalPayload([]byte(`{"payload":{"fieldOne":"valueOne","fieldTwo":"valueTwo"}}`), &v))
	suite.Equal(test{FieldOne: "valueOne", FieldTwo: "valueTwo"}, v)
	suite.ErrorIs(UnmarshalPayload([]byte(`{"loadplay":{"fieldOne":"valueOne","fieldTwo":"valueTwo"}}`), &v),
		ErrNotFound)
}

func (suite *MessagesTestSuite) TestUnmarshalEnvelope() {
	var e *Envelope
	var err error
	e, err = UnmarshalEnvelope([]byte(`{
		"timestamp": 1616935472,
		"type": "UserChangeResponse",
		"id": "my_id"}`))
	suite.NoError(err)
	suite.NotEmpty(e)

	e, err = UnmarshalEnvelope([]byte(`{
			"type": "UserChangeResponse",
			"id": "my_id"}`))
	suite.ErrorIs(err, ErrNotFound)
	suite.Empty(e)

	e, err = UnmarshalEnvelope([]byte(`{
			"timestamp": 1616935472,
			"id": "my_id"}`))
	suite.ErrorIs(err, ErrNotFound)
	suite.Empty(e)

	e, err = UnmarshalEnvelope([]byte(`{
			"timestamp": 1616935472,
			"type": "UserChangeResponse"}`))
	suite.ErrorIs(err, ErrNotFound)
	suite.Empty(e)
}

func (suite *MessagesTestSuite) TestWrap() {
	e := Envelope{
		Timestamp: 12345,
		Type:      "type",
		ID:        "ABCD-EF",
	}
	p := struct{ Field string }{"value"}
	m := Message{e, p}

	suite.Equal(m, e.Wrap(p))
}

func (suite *MessagesTestSuite) TestMarshal() {
	p := struct{ Field string }{"Value"}
	e := Envelope{Timestamp: 1616935472, Type: "UserChangeResponse", ID: "my_id"}
	m, err := e.Marshal(p)
	suite.NoError(err)
	suite.NotEmpty(m)
	suite.Equal(`{"Timestamp":1616935472,"Type":"UserChangeResponse","ID":"my_id","Payload":{"Field":"Value"}}`,
		string(m))
}
