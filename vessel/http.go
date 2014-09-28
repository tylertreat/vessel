package vessel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

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
	results   map[string]*result
	marshaler marshaler
}

func (h *httpHandler) sendHandler(w http.ResponseWriter, r *http.Request) {
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

	// TODO: Look into pluggable persistence.
	h.results[msg.ID] = &result{
		Done:      false,
		Responses: []*message{},
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
			r.Responses = append(r.Responses, &message{
				ID:      id,
				Channel: channel,
				Body:    result,
			})
		}
	}
}
