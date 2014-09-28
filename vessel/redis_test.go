package vessel

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockConn struct {
	mock.Mock
}

func (m *mockConn) Close() error {
	args := m.Mock.Called()
	return args.Error(0)
}

func (m *mockConn) Err() error {
	args := m.Mock.Called()
	return args.Error(0)
}

func (m *mockConn) Do(cmd string, a ...interface{}) (interface{}, error) {
	args := m.Mock.Called(cmd, a)
	return args.Get(0), args.Error(1)
}

func (m *mockConn) Send(cmd string, a ...interface{}) error {
	args := m.Mock.Called(cmd, a)
	return args.Error(0)
}

func (m *mockConn) Flush() error {
	args := m.Mock.Called()
	return args.Error(0)
}

func (m *mockConn) Receive() (interface{}, error) {
	args := m.Mock.Called()
	return args.Get(0), args.Error(1)
}

// Ensures that SaveResult performs a SET operation on redis and returns nil on
// success.
func TestSaveResult(t *testing.T) {
	mockConn := new(mockConn)
	r := &redisPersister{mu: sync.RWMutex{}, conn: mockConn}
	result := &result{Done: false, Responses: []*message{}}
	resultJSON, _ := json.Marshal(result)
	args := []interface{}{"abc", resultJSON}
	mockConn.On("Do", "SET", args).Return(nil, nil)

	err := r.SaveResult("abc", result)

	mockConn.Mock.AssertExpectations(t)
	assert.Nil(t, err)
}

// Ensures that SaveResult performs a SET operation on redis and returns an
// error on fail.
func TestSaveResultError(t *testing.T) {
	mockConn := new(mockConn)
	r := &redisPersister{mu: sync.RWMutex{}, conn: mockConn}
	result := &result{Done: false, Responses: []*message{}}
	resultJSON, _ := json.Marshal(result)
	args := []interface{}{"abc", resultJSON}
	mockConn.On("Do", "SET", args).Return(nil, fmt.Errorf("error"))

	err := r.SaveResult("abc", result)

	mockConn.Mock.AssertExpectations(t)
	assert.NotNil(t, err)
}

// Ensures that SaveMessage performs a SADD operation on redis and returns nil
// on success.
func TestSaveMessage(t *testing.T) {
	mockConn := new(mockConn)
	r := &redisPersister{mu: sync.RWMutex{}, conn: mockConn}
	message := &message{ID: "abc", Channel: "foo", Body: "bar"}
	messageJSON, _ := json.Marshal(message)
	args := []interface{}{"foo", messageJSON}
	mockConn.On("Do", "SADD", args).Return(nil, nil)

	err := r.SaveMessage("foo", message)

	mockConn.Mock.AssertExpectations(t)
	assert.Nil(t, err)
}

// Ensures that SaveMessage performs a SADD operation on redis and returns an
// error on fail.
func TestSaveMessageError(t *testing.T) {
	mockConn := new(mockConn)
	r := &redisPersister{mu: sync.RWMutex{}, conn: mockConn}
	message := &message{ID: "abc", Channel: "foo", Body: "bar"}
	messageJSON, _ := json.Marshal(message)
	args := []interface{}{"foo", messageJSON}
	mockConn.On("Do", "SADD", args).Return(nil, fmt.Errorf("error"))

	err := r.SaveMessage("foo", message)

	mockConn.Mock.AssertExpectations(t)
	assert.NotNil(t, err)
}

// Ensures that GetResult performs a GET operation on redis and returns the
// result.
func GetResult(t *testing.T) {
	assert := assert.New(t)
	mockConn := new(mockConn)
	r := &redisPersister{mu: sync.RWMutex{}, conn: mockConn}
	var res interface{}
	res, _ = json.Marshal(&result{Done: false, Responses: []*message{}})
	mockConn.On("Do", "GET", "abc").Return(res, nil)

	msg, err := r.GetResult("abc")

	mockConn.Mock.AssertExpectations(t)
	if assert.NotNil(msg) {
		assert.False(msg.Done)
		assert.Equal([]*message{}, msg.Responses)
	}
	assert.Nil(err)
}

// Ensures that GetResult performs a GET operation on redis and returns an
// error on fail.
func GetResultError(t *testing.T) {
	assert := assert.New(t)
	mockConn := new(mockConn)
	r := &redisPersister{mu: sync.RWMutex{}, conn: mockConn}
	mockConn.On("Do", "GET", "abc").Return(nil, fmt.Errorf("error"))

	msg, err := r.GetResult("abc")

	mockConn.Mock.AssertExpectations(t)
	assert.Nil(msg)
	assert.NotNil(err)
}

// Ensures that GetMessages performs a SMEMBERS operation on redis and returns
// the result.
func GetMessages(t *testing.T) {
	assert := assert.New(t)
	mockConn := new(mockConn)
	r := &redisPersister{mu: sync.RWMutex{}, conn: mockConn}
	var res interface{}
	msgJSON, _ := json.Marshal(&message{ID: "abc", Channel: "foo", Body: "bar"})
	res = [][]byte{msgJSON}
	mockConn.On("Do", "SMEMBERS", "foo").Return(res, nil)

	messages, err := r.GetMessages("foo")

	mockConn.Mock.AssertExpectations(t)
	if assert.NotNil(messages) {
		assert.Equal(1, len(messages))
		assert.Equal("abc", messages[0].ID)
		assert.Equal("foo", messages[0].Channel)
		assert.Equal("bar", messages[0].Body)
	}
	assert.Nil(err)
}

// Ensures that GetMessages performs a SMEMBERS operation on redis and returns
// an error on fail.
func GetMessagesError(t *testing.T) {
	assert := assert.New(t)
	mockConn := new(mockConn)
	r := &redisPersister{mu: sync.RWMutex{}, conn: mockConn}
	mockConn.On("Do", "SMEMBERS", "foo").Return(nil, fmt.Errorf("error"))

	messages, err := r.GetMessages("foo")

	mockConn.Mock.AssertExpectations(t)
	assert.Nil(messages)
	assert.NotNil(err)
}
