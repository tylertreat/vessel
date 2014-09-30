package vessel

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockSession struct {
	mock.Mock
}

func (m *mockSession) ID() string {
	args := m.Mock.Called()
	return args.String(0)
}

func (m *mockSession) Recv() (string, error) {
	args := m.Mock.Called()
	return args.String(0), args.Error(1)
}

func (m *mockSession) Send(msg string) error {
	args := m.Mock.Called(msg)
	return args.Error(0)
}

func (m *mockSession) Close(status uint32, reason string) error {
	args := m.Mock.Called(status, reason)
	return args.Error(0)
}

func mockIDGenerator() string {
	return "abc"
}

func newChannel(t *testing.T, expected bool) Channel {
	return func(name string, result chan<- string, done chan<- bool) {
		result <- "foo"
		done <- true
		if !expected {
			t.Errorf("Unexpected call to channel")
		}
	}
}

// Ensures that handler breaks and removes session when Recv fails.
func TestHandlerRecvError(t *testing.T) {
	assert := assert.New(t)
	session := new(mockSession)
	session.On("Recv").Return("", fmt.Errorf("error"))
	messages := make(chan *messageContext)
	transport := newSockJSTransport("/_vessel", messages)
	handler := transport.(*sockjsTransport).handler()

	handler(session)

	assert.Equal(0, len(transport.(*sockjsTransport).sessions))
	session.Mock.AssertExpectations(t)
}

// Ensures that handler doesn't ship anything out if unmarshal fails.
func TestHandlerBadMessage(t *testing.T) {
	assert := assert.New(t)
	session := new(mockSession)
	session.On("Recv").Return(`{"foo": "bar"`, nil).Once()
	session.On("Recv").Return("", fmt.Errorf("error")).Once()
	messages := make(chan *messageContext)
	transport := newSockJSTransport("/_vessel", messages)
	handler := transport.(*sockjsTransport).handler()

	handler(session)

	assert.Equal(0, len(transport.(*sockjsTransport).sessions))
	session.Mock.AssertExpectations(t)
}

// Ensures that handler ships out messages for processing.
func TestHandlerChannel(t *testing.T) {
	assert := assert.New(t)
	session := new(mockSession)
	session.On("Recv").Return(
		`{"channel": "foo", "id": "abc", "body": "foobar", "timestamp": 1412003438}`, nil).Once()
	session.On("Recv").Return("", fmt.Errorf("error")).Once()
	messages := make(chan *messageContext, 1)
	transport := newSockJSTransport("/_vessel", messages)
	handler := transport.(*sockjsTransport).handler()

	handler(session)

	assert.Equal(0, len(transport.(*sockjsTransport).sessions))
	msgCtx := <-messages
	msg := msgCtx.message
	assert.Equal("abc", msg.ID)
	assert.Equal("foo", msg.Channel)
	assert.Equal("foobar", msg.Body)
	assert.Equal(1412003438, msg.Timestamp)
}

// Ensures that publish sends on all sessions.
func TestPublish(t *testing.T) {
	session1 := new(mockSession)
	session2 := new(mockSession)
	session1.On("Send", `{"id":"abc","channel":"foo","body":"bar","timestamp":1412003438}`).Return(nil)
	session2.On("Send", `{"id":"abc","channel":"foo","body":"bar","timestamp":1412003438}`).Return(nil)
	transport := newSockJSTransport("/_vessel", make(chan *messageContext))
	transport.(*sockjsTransport).sessions = append(transport.(*sockjsTransport).sessions, session1, session2)

	transport.publish(&message{ID: "abc", Channel: "foo", Body: "bar", Timestamp: 1412003438})

	session1.Mock.AssertExpectations(t)
	session2.Mock.AssertExpectations(t)
}
