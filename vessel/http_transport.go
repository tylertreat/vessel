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

type httpTransport struct {
	marshaler marshaler
	uri       string
	messages  chan<- *messageContext
	persister Persister
}

func newHTTPTransport(uri string, messages chan<- *messageContext, persister Persister) transport {
	transport := &httpTransport{
		marshaler: &jsonMarshaler{},
		uri:       uri,
		messages:  messages,
		persister: persister,
	}
	return transport
}

func (h *httpTransport) listenAndServe(portStr string) error {
	r := mux.NewRouter()
	r.HandleFunc(h.uri, h.send).Methods("POST")
	r.HandleFunc(h.uri+"/message/{id}", h.pollResponses).Methods("GET")
	r.HandleFunc(h.uri+"/channel/{channel}", h.pollSubscription).Methods("GET")
	http.Handle("/", &httpProxy{r})
	return http.ListenAndServe(portStr, nil)
}

// Noop. Message has been persisted at this point.
func (h *httpTransport) publish(*message) {}

type httpProxy struct {
	r *mux.Router
}

func (h *httpProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
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

// send allows HTTP clients to send messages into the system. It calls Recv on
// messages to invoke channel handlers and begins dispatching responses.
// Responses can be polled using the pollResponses handler.
func (h *httpTransport) send(w http.ResponseWriter, r *http.Request) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)

	msg, err := h.marshaler.unmarshal(buf.Bytes())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	msgCtx := &messageContext{msg, func(id, channel string, c <-chan string, done <-chan bool) {
		h.dispatch(id, channel, c, done)
	}}

	// Ship out message.
	h.messages <- msgCtx

	result := &result{
		Done:      false,
		Responses: []*message{},
	}
	h.persister.SaveResult(msg.ID, result)

	var scheme string
	scheme = r.URL.Scheme
	if scheme == "" {
		scheme = "http"
	}

	urlStr := fmt.Sprintf("%s://%s%s/message/%s", scheme, r.Host, h.uri, msg.ID)

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
func (h *httpTransport) pollResponses(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	result, err := h.persister.GetResult(id)
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
func (h *httpTransport) pollSubscription(w http.ResponseWriter, r *http.Request) {
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

	messages, err := h.persister.GetMessages(channel, since)
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
func (h *httpTransport) dispatch(id, channel string, results <-chan string, done <-chan bool) {
	r, err := h.persister.GetResult(id)
	if err != nil {
		log.Println(err)
		return
	}

	for {
		select {
		case <-done:
			r.Done = true
			h.persister.SaveResult(id, r)
		case result := <-results:
			r.Responses = append(r.Responses, newMessage(id, channel, result))
			h.persister.SaveResult(id, r)
		}
	}
}
