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

func (m *mockPersister) GetMessages(channel string, since int64) ([]*message, error) {
	args := m.Mock.Called(channel, since)
	messages := args.Get(0)
	if messages != nil {
		return messages.([]*message), args.Error(1)
	}
	return nil, args.Error(1)
}

// Ensures that send writes an error message when the payload is bad.
func TestSendBadRequest(t *testing.T) {
	assert := assert.New(t)
	mockPersister := new(mockPersister)
	messages := make(chan *messageContext)
	transport := newHTTPTransport("/_vessel", messages, mockPersister)
	w := httptest.NewRecorder()
	payload := map[string]interface{}{
		"channel": "foo",
		"body":    "bar",
	}
	jsonPayload, _ := json.Marshal(payload)
	reader := bytes.NewReader(jsonPayload)
	req, _ := http.NewRequest("POST", "http://example.com/vessel", reader)

	transport.(*httpTransport).send(w, req)

	assert.Equal(http.StatusBadRequest, w.Code)
	assert.Equal("Message missing id", w.Body.String())
}

// Ensures that send dispatches the message and writes the resource URL.
func TestSend(t *testing.T) {
	assert := assert.New(t)
	mockPersister := new(mockPersister)
	messages := make(chan *messageContext, 1)
	transport := newHTTPTransport("/_vessel", messages, mockPersister)
	w := httptest.NewRecorder()
	payload := map[string]interface{}{
		"id":        "abc",
		"channel":   "foo",
		"body":      "bar",
		"timestamp": 1412003438,
	}
	jsonPayload, _ := json.Marshal(payload)
	reader := bytes.NewReader(jsonPayload)
	req, _ := http.NewRequest("POST", "http://example.com/_vessel", reader)
	result := &result{Done: false, Responses: []*message{}}
	mockPersister.On("SaveResult", "abc", result).Return(nil)

	transport.(*httpTransport).send(w, req)

	msgCtx := <-messages
	msg := msgCtx.message
	assert.Equal("abc", msg.ID)
	assert.Equal("foo", msg.Channel)
	assert.Equal("bar", msg.Body)
	assert.Equal(1412003438, msg.Timestamp)
	assert.Equal(http.StatusAccepted, w.Code)
	assert.Equal(
		`{"channel":"foo","id":"abc","responses":"http://example.com/_vessel/message/abc"}`,
		w.Body.String())
}

// Ensures that pollResponses writes an error message when there is no message.
func TestPollResponsesNoMessage(t *testing.T) {
	assert := assert.New(t)
	mockPersister := new(mockPersister)
	transport := newHTTPTransport("/_vessel", make(chan *messageContext), mockPersister)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://example.com/_vessel/message/abc", nil)
	r := router(transport.(*httpTransport).pollResponses)
	mockPersister.On("GetResult", "abc").Return(nil, fmt.Errorf("no result"))

	r.ServeHTTP(w, req)

	assert.Equal(http.StatusNotFound, w.Code)
	assert.Equal("", w.Body.String())
}

// Ensures that pollResponses returns a JSON payload containing the message responses.
func TestPollHandler(t *testing.T) {
	assert := assert.New(t)
	mockPersister := new(mockPersister)
	transport := newHTTPTransport("/_vessel", make(chan *messageContext), mockPersister)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://example.com/_vessel/message/abc", nil)
	r := router(transport.(*httpTransport).pollResponses)
	result := &result{
		Done:      true,
		Responses: []*message{&message{ID: "abc", Channel: "foo", Body: "bar", Timestamp: 1412003438}},
	}
	mockPersister.On("GetResult", "abc").Return(result, nil)

	r.ServeHTTP(w, req)

	assert.Equal(http.StatusOK, w.Code)
	assert.Equal(
		`{"done":true,"responses":[{"id":"abc","channel":"foo","body":"bar","timestamp":1412003438}]}`,
		w.Body.String(),
	)
}

func router(handler http.HandlerFunc) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/_vessel/message/{id}", handler)
	return r
}
