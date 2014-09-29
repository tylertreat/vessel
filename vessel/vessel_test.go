package vessel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockVessel struct {
	mock.Mock
}

func (m *mockVessel) AddChannel(name string, channel Channel) {
	m.Mock.Called(name, channel)
}

func (m *mockVessel) Start(sockPortStr, httpPortStr string) error {
	args := m.Mock.Called(sockPortStr, httpPortStr)
	return args.Error(0)
}

func (m *mockVessel) Recv(msg *message) (<-chan string, <-chan bool, error) {
	args := m.Mock.Called(msg)
	return args.Get(0).(<-chan string), args.Get(1).(<-chan bool), args.Error(2)
}

func (m *mockVessel) Broadcast(channel string, msg string) {
	m.Mock.Called(channel, msg)
}

func (m *mockVessel) Persister() Persister {
	args := m.Mock.Called()
	return args.Get(0).(Persister)
}

func (m *mockVessel) URI() string {
	args := m.Mock.Called()
	return args.String(0)
}

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

// Ensures that unmarshal returns nil and an error when the message is missing a timestamp.
func TestUnmarshalMissingTimestamp(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}

	message, err := j.unmarshal([]byte(`{"id": "abc", "channel": "foo", "body": "bar"}`))

	assert.Nil(message)
	assert.NotNil(err)
}

// Ensures that unmarshal returns the expected message when valid JSON is provided.
func TestUnmarshalHappyPath(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}

	message, err := j.unmarshal(
		[]byte(`{"id": "abc", "channel": "foo", "body": "bar", "timestamp": 1412003438}`))

	if assert.NotNil(message) {
		assert.Equal("abc", message.ID)
		assert.Equal("foo", message.Channel)
		assert.Equal("bar", message.Body)
	}
	assert.Nil(err)
}

// Ensures that marshal returns the expected message JSON.
func TestMarshal(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}
	message := &message{ID: "foo", Channel: "bar", Body: "baz", Timestamp: 1412003438}

	messageJSON, err := j.marshal(message)

	assert.Equal(`{"id":"foo","channel":"bar","body":"baz","timestamp":1412003438}`,
		string(messageJSON))
	assert.Nil(err)
}
