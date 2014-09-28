package vessel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
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

type mockPersister struct {
	mock.Mock
}

func (m *mockPersister) Prepare() error {
	args := m.Mock.Called()
	return args.Error(0)
}

func (m *mockPersister) SaveResult(id string, result *result) error {
	args := m.Mock.Called(id, result)
	return args.Error(0)
}

func (m *mockPersister) SaveMessage(id string, message *message) error {
	args := m.Mock.Called(id, message)
	return args.Error(0)
}

func (m *mockPersister) GetResult(id string) (*result, error) {
	args := m.Mock.Called(id)
	r := args.Get(0)
	if r != nil {
		return r.(*result), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPersister) GetMessages(channel string) ([]*message, error) {
	args := m.Mock.Called(channel)
	messages := args.Get(0)
	if messages != nil {
		return messages.([]*message), args.Error(1)
	}
	return nil, args.Error(1)
}

// Ensures that send writes an error message when the payload is bad.
func TestSendBadRequest(t *testing.T) {
	assert := assert.New(t)
	mockVessel := new(mockVessel)
	handler := newHTTPHandler(mockVessel)
	w := httptest.NewRecorder()
	payload := map[string]interface{}{
		"channel": "foo",
		"body":    "bar",
	}
	jsonPayload, _ := json.Marshal(payload)
	reader := bytes.NewReader(jsonPayload)
	req, _ := http.NewRequest("POST", "http://example.com/_vessel", reader)

	handler.send(w, req)

	assert.Equal(http.StatusBadRequest, w.Code)
	assert.Equal("Message missing id", w.Body.String())
}

// Ensures that send writes an error message when Recv fails.
func TestSendRecvFail(t *testing.T) {
	assert := assert.New(t)
	mockVessel := new(mockVessel)
	handler := newHTTPHandler(mockVessel)
	w := httptest.NewRecorder()
	payload := map[string]interface{}{
		"id":      "abc",
		"channel": "foo",
		"body":    "bar",
	}
	jsonPayload, _ := json.Marshal(payload)
	reader := bytes.NewReader(jsonPayload)
	req, _ := http.NewRequest("POST", "http://example.com/_vessel", reader)
	mockVessel.On("Recv", &message{ID: "abc", Channel: "foo", Body: "bar"}).
		Return(make(<-chan string), make(<-chan bool), fmt.Errorf("error"))

	handler.send(w, req)

	mockVessel.Mock.AssertExpectations(t)
	assert.Equal(http.StatusInternalServerError, w.Code)
	assert.Equal("error", w.Body.String())
}

// Ensures that send dispatches the message and writes the resource URL.
func TestSend(t *testing.T) {
	assert := assert.New(t)
	mockVessel := new(mockVessel)
	mockPersister := new(mockPersister)
	handler := newHTTPHandler(mockVessel)
	w := httptest.NewRecorder()
	payload := map[string]interface{}{
		"id":      "abc",
		"channel": "foo",
		"body":    "bar",
	}
	jsonPayload, _ := json.Marshal(payload)
	reader := bytes.NewReader(jsonPayload)
	req, _ := http.NewRequest("POST", "http://example.com/_vessel", reader)
	mockVessel.On("Recv", &message{ID: "abc", Channel: "foo", Body: "bar"}).
		Return(make(<-chan string), make(<-chan bool), nil)
	mockVessel.On("Persister").Return(mockPersister)
	result := &result{Done: false, Responses: []*message{}}
	mockPersister.On("SaveResult", "abc", result).Return(nil)
	mockPersister.On("GetResult", "abc").Return(nil, fmt.Errorf("error"))

	handler.send(w, req)

	mockVessel.Mock.AssertExpectations(t)
	assert.Equal(http.StatusAccepted, w.Code)
	assert.Equal(
		`{"channel":"foo","id":"abc","responses":"http://example.com/_vessel/message/abc"}`,
		w.Body.String())
}

// Ensures that pollResponses writes an error message when there is no message.
func TestPollResponsesNoMessage(t *testing.T) {
	assert := assert.New(t)
	mockVessel := new(mockVessel)
	mockPersister := new(mockPersister)
	handler := newHTTPHandler(mockVessel)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://example.com/_vessel/message/abc", nil)
	r := router(handler.pollResponses)
	mockVessel.On("Persister").Return(mockPersister)
	mockPersister.On("GetResult", "abc").Return(nil, fmt.Errorf("no result"))

	r.ServeHTTP(w, req)

	assert.Equal(http.StatusNotFound, w.Code)
	assert.Equal("", w.Body.String())
}

// Ensures that pollResponses returns a JSON payload containing the message responses.
func TestPollHandler(t *testing.T) {
	assert := assert.New(t)
	mockVessel := new(mockVessel)
	mockPersister := new(mockPersister)
	handler := newHTTPHandler(mockVessel)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://example.com/_vessel/message/abc", nil)
	r := router(handler.pollResponses)
	mockVessel.On("Persister").Return(mockPersister)
	result := &result{
		Done:      true,
		Responses: []*message{&message{ID: "abc", Channel: "foo", Body: "bar"}},
	}
	mockPersister.On("GetResult", "abc").Return(result, nil)

	r.ServeHTTP(w, req)

	assert.Equal(http.StatusOK, w.Code)
	assert.Equal(`{"done":true,"responses":[{"id":"abc","channel":"foo","body":"bar"}]}`, w.Body.String())
}

func router(handler http.HandlerFunc) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/_vessel/message/{id}", handler)
	return r
}
