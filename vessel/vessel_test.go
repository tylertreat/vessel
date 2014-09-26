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

func (m *mockVessel) Marshaler() Marshaler {
	args := m.Mock.Called()
	return args.Get(0).(Marshaler)
}

// Ensures that unmarshal returns nil and an error when the message is not valid JSON.
func TestUnmarshalBadJSON(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}

	message, err := j.Unmarshal([]byte(`{"foo":}`))

	assert.Nil(message)
	assert.NotNil(err)
}

// Ensures that unmarshal returns nil and an error when the message is missing an ID.
func TestUnmarshalMissingID(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}

	message, err := j.Unmarshal([]byte(`{"channel": "foo", "body": "bar"}`))

	assert.Nil(message)
	assert.NotNil(err)
}

// Ensures that unmarshal returns nil and an error when the message is missing a channel.
func TestUnmarshalMissingChannel(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}

	message, err := j.Unmarshal([]byte(`{"id": "foo", "body": "bar"}`))

	assert.Nil(message)
	assert.NotNil(err)
}

// Ensures that unmarshal returns nil and an error when the message is missing a body.
func TestUnmarshalMissingBody(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}

	message, err := j.Unmarshal([]byte(`{"id": "foo", "channel": "bar"}`))

	assert.Nil(message)
	assert.NotNil(err)
}

// Ensures that unmarshal returns the expected message when valid JSON is provided.
func TestUnmarshalHappyPath(t *testing.T) {
	assert := assert.New(t)
	j := &jsonMarshaler{}

	message, err := j.Unmarshal([]byte(`{"id": "abc", "channel": "foo", "body": "bar"}`))

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
	message := &message{ID: "foo", Channel: "bar", Body: "baz"}

	messageJSON, err := j.Marshal(message)

	assert.Equal(`{"id":"foo","channel":"bar","body":"baz"}`, string(messageJSON))
	assert.Nil(err)
}

// Ensures that sendHandler writes an error message when the payload is bad.
func TestSendHandlerBadRequest(t *testing.T) {
	assert := assert.New(t)
	mockVessel := new(mockVessel)
	handler := &httpHandler{mockVessel, map[string]*result{}, &jsonMarshaler{}}
	w := httptest.NewRecorder()
	payload := map[string]interface{}{
		"channel": "foo",
		"body":    "bar",
	}
	jsonPayload, _ := json.Marshal(payload)
	reader := bytes.NewReader(jsonPayload)
	req, _ := http.NewRequest("POST", "http://example.com/_vessel", reader)

	handler.sendHandler(w, req)

	assert.Equal(http.StatusBadRequest, w.Code)
	assert.Equal("Message missing id", w.Body.String())
}

// Ensures that sendHandler writes an error message when Recv fails.
func TestSendHandlerRecvFail(t *testing.T) {
	assert := assert.New(t)
	mockVessel := new(mockVessel)
	handler := &httpHandler{mockVessel, map[string]*result{}, &jsonMarshaler{}}
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

	handler.sendHandler(w, req)

	mockVessel.Mock.AssertExpectations(t)
	assert.Equal(http.StatusInternalServerError, w.Code)
	assert.Equal("error", w.Body.String())
}

// Ensures that sendHandler dispatches the message and writes the resource URL.
func TestSendHandler(t *testing.T) {
	assert := assert.New(t)
	mockVessel := new(mockVessel)
	handler := &httpHandler{mockVessel, map[string]*result{}, &jsonMarshaler{}}
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

	handler.sendHandler(w, req)

	mockVessel.Mock.AssertExpectations(t)
	assert.Equal(http.StatusAccepted, w.Code)
	assert.Equal(
		`{"channel":"foo","id":"abc","responses":"http://example.com/_vessel/message/abc"}`,
		w.Body.String())
	result := handler.results["abc"]
	if assert.NotNil(result) {
		assert.False(result.Done)
		assert.Equal([]*message{}, result.Results)
	}
}

// Ensures that pollHandler writes an error message when there is no message.
func TestPollHandlerNoMessage(t *testing.T) {
	assert := assert.New(t)
	mockVessel := new(mockVessel)
	handler := &httpHandler{mockVessel, map[string]*result{}, &jsonMarshaler{}}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://example.com/_vessel/message/abc", nil)

	handler.pollHandler(w, req)

	assert.Equal(http.StatusNotFound, w.Code)
	assert.Equal("", w.Body.String())
}

// Ensures that pollHandler returns a JSON payload containing the message responses.
func TestPollHandler(t *testing.T) {
	assert := assert.New(t)
	mockVessel := new(mockVessel)
	handler := &httpHandler{mockVessel, map[string]*result{}, &jsonMarshaler{}}
	handler.results["abc"] = &result{
		Done:    true,
		Results: []*message{&message{ID: "abc", Channel: "foo", Body: "bar"}},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://example.com/_vessel/message/abc", nil)
	r := router(handler.pollHandler)

	r.ServeHTTP(w, req)

	assert.Equal(http.StatusOK, w.Code)
	assert.Equal(`{"done":true,"results":[{"id":"abc","channel":"foo","body":"bar"}]}`, w.Body.String())
}

func router(handler http.HandlerFunc) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/_vessel/message/{id}", handler)
	return r
}
