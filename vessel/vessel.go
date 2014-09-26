package vessel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mattrobenolt/gocql/uuid"
)

// Channel is a function which takes a message, a channel for sending results, and a channel
// for signaling that the handler has completed.
type Channel func(string, chan<- string, chan<- bool)

// Vessel coordinates communication between clients and server. It's responsible for managing
// Channels and processing incoming and outgoing messages.
type Vessel interface {
	// AddChannel registers the Channel handler with the specified name.
	AddChannel(string, Channel)

	// Start will start the server on the given ports.
	Start(string, string) error

	Recv(*message) (<-chan string, <-chan bool, error)

	// Broadcast sends the specified message on the given channel to all connected clients.
	Broadcast(string, string)

	Marshaler() Marshaler
}

type message struct {
	ID      string `json:"id"`
	Channel string `json:"channel"`
	Body    string `json:"body"`
}

type Marshaler interface {
	Unmarshal([]byte) (*message, error)
	Marshal(*message) ([]byte, error)
}

type jsonMarshaler struct{}

func (j *jsonMarshaler) Unmarshal(msg []byte) (*message, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(msg, &payload); err != nil {
		return nil, err
	}

	id, ok := payload["id"]
	if !ok {
		return nil, fmt.Errorf("Message missing id")
	}

	channel, ok := payload["channel"]
	if !ok {
		return nil, fmt.Errorf("Message missing channel")
	}

	body, ok := payload["body"]
	if !ok {
		return nil, fmt.Errorf("Message missing body")
	}

	message := &message{
		ID:      id.(string),
		Channel: channel.(string),
		Body:    body.(string),
	}

	return message, nil
}

func (j *jsonMarshaler) Marshal(message *message) ([]byte, error) {
	return json.Marshal(message)
}

type idGenerator interface {
	generate() string
}

type uuidGenerator struct{}

func (u *uuidGenerator) generate() string {
	uuid := uuid.RandomUUID().String()
	return strings.Replace(uuid, "-", "", -1)
}

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
	Done    bool       `json:"done"`
	Results []*message `json:"results"`
}

type httpHandler struct {
	Vessel
	results   map[string]*result
	marshaler Marshaler
}

func (h *httpHandler) sendHandler(w http.ResponseWriter, r *http.Request) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)

	msg, err := h.marshaler.Unmarshal(buf.Bytes())
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

	// TODO: Look into pluggable persistence.
	h.results[msg.ID] = &result{
		Done:    false,
		Results: []*message{},
	}

	go h.dispatch(msg.ID, msg.Channel, results, done)

	var scheme string
	scheme = r.URL.Scheme
	if scheme == "" {
		scheme = "http"
	}

	urlStr := fmt.Sprintf("%s://%s/_vessel/message/%s", scheme, r.Host, msg.ID)

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

func (h *httpHandler) pollHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	result, ok := h.results[id]
	if !ok {
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

func (h *httpHandler) dispatch(id, channel string, results <-chan string, done <-chan bool) {
	r := h.results[id]
	for {
		select {
		case <-done:
			r.Done = true
		case result := <-results:
			r.Results = append(r.Results, &message{
				ID:      id,
				Channel: channel,
				Body:    result,
			})
		}
	}
}
