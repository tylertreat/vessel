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

type mockIDGenerator struct {
	mock.Mock
}

func (m *mockIDGenerator) generate() string {
	args := m.Mock.Called()
	return args.String(0)
}

func newChannel(called *bool) Channel {
	return func(name string, result chan<- string, done chan<- bool) {
		*called = true
		result <- "foo"
		done <- true
	}
}

// Ensures that sockjsVessel handler breaks and doesn't call Channel when Recv fails.
func TestHandlerRecvError(t *testing.T) {
	assert := assert.New(t)
	session := new(mockSession)
	session.On("Recv").Return("", fmt.Errorf("error"))
	vessel := NewSockJSVessel("http://localhost.com/foo")
	called := false
	vessel.AddChannel("foo", newChannel(&called))
	handler := vessel.(*sockjsVessel).handler()

	handler(session)

	assert.False(called)
	assert.Equal(0, len(vessel.(*sockjsVessel).sessions))
	session.Mock.AssertExpectations(t)
}

// Ensures that sockjsVessel handler doesn't call Channel when unmarshal fails.
func TestHandlerBadMessage(t *testing.T) {
	assert := assert.New(t)
	session := new(mockSession)
	session.On("Recv").Return(`{"foo": "bar"`, nil).Once()
	session.On("Recv").Return("", fmt.Errorf("error")).Once()
	vessel := NewSockJSVessel("http://localhost.com/foo")
	called := false
	vessel.AddChannel("foo", newChannel(&called))
	handler := vessel.(*sockjsVessel).handler()

	handler(session)

	assert.False(called)
	assert.Equal(0, len(vessel.(*sockjsVessel).sessions))
	session.Mock.AssertExpectations(t)
}

// Ensures that sockjsVessel handler calls the Channel and sends its results.
func TestHandlerChannel(t *testing.T) {
	assert := assert.New(t)
	session := new(mockSession)
	session.On("Recv").Return(`{"channel": "foo", "id": "abc", "body": "foobar"}`, nil).Once()
	session.On("Recv").Return("", fmt.Errorf("error")).Once()
	session.On("Send", `{"id":"abc","channel":"foo","body":"foo"}`).Return(nil)
	vessel := NewSockJSVessel("http://localhost.com/foo")
	called := false
	vessel.AddChannel("foo", newChannel(&called))
	handler := vessel.(*sockjsVessel).handler()

	handler(session)

	assert.True(called)
	assert.Equal(0, len(vessel.(*sockjsVessel).sessions))
	session.Mock.AssertExpectations(t)
}

// Ensures that Broadcast sends on all sessions.
func TestBroadcast(t *testing.T) {
	session1 := new(mockSession)
	session2 := new(mockSession)
	idGenerator := new(mockIDGenerator)
	session1.On("Send", `{"id":"abc","channel":"foo","body":"bar"}`).Return(nil)
	session2.On("Send", `{"id":"abc","channel":"foo","body":"bar"}`).Return(nil)
	idGenerator.On("generate").Return("abc")
	vessel := NewSockJSVessel("http://localhost.com/foo")
	vessel.(*sockjsVessel).idGenerator = idGenerator
	vessel.(*sockjsVessel).sessions = append(vessel.(*sockjsVessel).sessions, session1, session2)

	vessel.Broadcast("foo", "bar")

	session1.Mock.AssertExpectations(t)
	session2.Mock.AssertExpectations(t)
}
