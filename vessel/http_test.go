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
)

// Ensures that sendHandler writes an error message when the payload is bad.
func TestSendHandlerBadRequest(t *testing.T) {
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

	handler.sendHandler(w, req)

	assert.Equal(http.StatusBadRequest, w.Code)
	assert.Equal("Message missing id", w.Body.String())
}

// Ensures that sendHandler writes an error message when Recv fails.
func TestSendHandlerRecvFail(t *testing.T) {
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

	handler.sendHandler(w, req)

	mockVessel.Mock.AssertExpectations(t)
	assert.Equal(http.StatusInternalServerError, w.Code)
	assert.Equal("error", w.Body.String())
}

// Ensures that sendHandler dispatches the message and writes the resource URL.
func TestSendHandler(t *testing.T) {
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
		assert.Equal([]*message{}, result.Responses)
	}
}

// Ensures that pollHandler writes an error message when there is no message.
func TestPollHandlerNoMessage(t *testing.T) {
	assert := assert.New(t)
	mockVessel := new(mockVessel)
	handler := newHTTPHandler(mockVessel)
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
	handler := newHTTPHandler(mockVessel)
	handler.results["abc"] = &result{
		Done:      true,
		Responses: []*message{&message{ID: "abc", Channel: "foo", Body: "bar"}},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://example.com/_vessel/message/abc", nil)
	r := router(handler.pollHandler)

	r.ServeHTTP(w, req)

	assert.Equal(http.StatusOK, w.Code)
	assert.Equal(`{"done":true,"responses":[{"id":"abc","channel":"foo","body":"bar"}]}`, w.Body.String())
}

func router(handler http.HandlerFunc) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/_vessel/message/{id}", handler)
	return r
}
