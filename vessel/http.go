package vessel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type httpServer struct {
	r *mux.Router
}

func (h *httpServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if origin := req.Header.Get("Origin"); origin != "" {
		rw.Header().Set("Access-Control-Allow-Origin", origin)
		rw.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		rw.Header().Set("Access-Control-Allow-Headers",
			"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	}

	if req.Method == "OPTIONS" {
		return
	}

	h.r.ServeHTTP(rw, req)
}

type result struct {
	Done      bool       `json:"done"`
	Responses []*message `json:"responses"`
}

type httpHandler struct {
	Vessel
	marshaler *jsonMarshaler
}

func newHTTPHandler(vessel Vessel) *httpHandler {
	return &httpHandler{
		vessel,
		&jsonMarshaler{},
	}
}

// send allows HTTP clients to send messages into the system. It calls Recv on
// messages to invoke channel handlers and begins dispatching responses.
// Responses can be polled using the pollResponses handler.
func (h *httpHandler) send(w http.ResponseWriter, r *http.Request) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)

	msg, err := h.marshaler.unmarshal(buf.Bytes())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	results, done, err := h.Recv(msg)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	result := &result{
		Done:      false,
		Responses: []*message{},
	}
	h.Persister().SaveResult(msg.ID, result)

	go h.dispatch(msg.ID, msg.Channel, results, done)

	var scheme string
	scheme = r.URL.Scheme
	if scheme == "" {
		scheme = "http"
	}

	urlStr := fmt.Sprintf("%s://%s%s/message/%s", scheme, r.Host, h.URI(), msg.ID)

	payload := map[string]interface{}{
		"id":        msg.ID,
		"channel":   msg.Channel,
		"responses": urlStr,
	}
	resp, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write(resp)
}

// pollResponses will return any responses messages for the message with the
// given id.
func (h *httpHandler) pollResponses(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	result, err := h.Persister().GetResult(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	resp, err := json.Marshal(result)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// pollSubscriptions will return all messages on a channel since the provided
// timestamp.
func (h *httpHandler) pollSubscription(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	channel := vars["channel"]
	var since int64
	if sinceStr, ok := r.URL.Query()["since"]; ok {
		var err error
		since, err = strconv.ParseInt(sinceStr[0], 0, 64)
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
	}

	messages, err := h.Persister().GetMessages(channel, since)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	resp, err := json.Marshal(messages)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// dispatch will listen for responses to a message and add them to the message
// result struct for polling.
func (h *httpHandler) dispatch(id, channel string, results <-chan string, done <-chan bool) {
	persister := h.Persister()
	r, err := persister.GetResult(id)
	if err != nil {
		log.Println(err)
		return
	}

	for {
		select {
		case <-done:
			r.Done = true
			persister.SaveResult(id, r)
		case result := <-results:
			r.Responses = append(r.Responses, newMessage(id, channel, result))
			persister.SaveResult(id, r)
		}
	}
}
