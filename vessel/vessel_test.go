package vessel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Ensures that unmarshal returns nil and an error when the message is not valid JSON.
func TestUnmarshalBadJSON(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}

	message, err := j.unmarshal([]byte(`{"foo":}`))

	assert.Nil(message)
	assert.NotNil(err)
}

// Ensures that unmarshal returns nil and an error when the message is missing an ID.
func TestUnmarshalMissingID(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}

	message, err := j.unmarshal([]byte(`{"channel": "foo", "body": "bar"}`))

	assert.Nil(message)
	assert.NotNil(err)
}

// Ensures that unmarshal returns nil and an error when the message is missing a channel.
func TestUnmarshalMissingChannel(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}

	message, err := j.unmarshal([]byte(`{"id": "foo", "body": "bar"}`))

	assert.Nil(message)
	assert.NotNil(err)
}

// Ensures that unmarshal returns nil and an error when the message is missing a body.
func TestUnmarshalMissingBody(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}

	message, err := j.unmarshal([]byte(`{"id": "foo", "channel": "bar"}`))

	assert.Nil(message)
	assert.NotNil(err)
}

// Ensures that unmarshal returns the expected message when valid JSON is provided.
func TestUnmarshalHappyPath(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}

	message, err := j.unmarshal([]byte(`{"id": "abc", "channel": "foo", "body": "bar"}`))

	if assert.NotNil(message) {
		assert.Equal("abc", message.ID)
		assert.Equal("foo", message.Channel)
		assert.Equal("bar", message.Body)
	}
	assert.Nil(err)
}

func TestMarshal(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}
	message := &message{ID: "foo", Channel: "bar", Body: "baz"}

	messageJSON, err := j.marshal(message)

	assert.Equal(`{"id":"foo","channel":"bar","body":"baz"}`, string(messageJSON))
	assert.Nil(err)
}
