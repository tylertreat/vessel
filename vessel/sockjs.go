package vessel

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/igm/sockjs-go/sockjs"
)

type sockjsVessel struct {
	uri         string
	sessions    []sockjs.Session
	channels    map[string]Channel
	marshaler   Marshaler
	idGenerator idGenerator
	httpHandler *httpHandler
}

// NewSockJSVessel returns a new Vessel which relies on SockJS as the underlying transport.
func NewSockJSVessel(uri string) Vessel {
	marshaler := &jsonMarshaler{}
	vessel := &sockjsVessel{
		uri:         uri,
		channels:    map[string]Channel{},
		sessions:    []sockjs.Session{},
		marshaler:   marshaler,
		idGenerator: &uuidGenerator{},
	}
	httpHandler := &httpHandler{vessel, map[string][]*message{}, map[string]*result{}}
	vessel.httpHandler = httpHandler
	return vessel
}

// AddChannel registers the Channel handler with the specified name.
func (v *sockjsVessel) AddChannel(name string, channel Channel) {
	v.channels[name] = channel
}

// Start will start the server on the given port.
func (v *sockjsVessel) Start(sockPortStr, httpPortStr string) error {
	sockjsHandler := sockjs.NewHandler(v.uri, sockjs.DefaultOptions, v.handler())
	r := mux.NewRouter()
	r.HandleFunc("/_vessel", v.httpHandler.sendHandler).Methods("POST")
	r.HandleFunc("/_vessel/message/{id}", v.httpHandler.pollHandler).Methods("GET")
	http.Handle("/", r)
	go func() {
		http.ListenAndServe(httpPortStr, r)
	}()
	return http.ListenAndServe(sockPortStr, sockjsHandler)
}

func (v *sockjsVessel) Marshaler() Marshaler {
	return v.marshaler
}

// Broadcast sends the specified message on the given channel to all connected clients.
func (s *sockjsVessel) Broadcast(channel string, msg string) {
	m := &message{
		ID:      s.idGenerator.generate(),
		Channel: channel,
		Body:    msg,
	}

	for _, session := range s.sessions {
		if send, err := s.marshaler.Marshal(m); err != nil {
			log.Println(err)
		} else {
			sendStr := string(send)
			log.Println("Send", sendStr)
			session.Send(sendStr)
		}
	}
}

func (s *sockjsVessel) handler() func(sockjs.Session) {
	return func(session sockjs.Session) {
		s.sessions = append(s.sessions, session)

		for {
			msg, err := session.Recv()
			if err != nil {
				log.Println(err)
				break
			}

			// Process message and invoke handler for it.
			recvMsg, results, done, err := s.Recv([]byte(msg))
			if err != nil {
				log.Println(err)
				continue
			}

			// Begin dispatching results produced by the handler.
			go s.dispatchResponses(recvMsg.ID, recvMsg.Channel, results, done, session)
		}

		// Remove session from Vessel.
		for i, sess := range s.sessions {
			if sess == session {
				s.sessions = append(s.sessions[:i], s.sessions[i+1:]...)
				break
			}
		}
	}

}

func (s *sockjsVessel) Recv(msg []byte) (*message, <-chan string, <-chan bool, error) {
	log.Println("Recv", msg)
	recvMsg, err := s.marshaler.Unmarshal(msg)
	if err != nil {
		return nil, nil, nil, err
	}

	channelHandler, ok := s.channels[recvMsg.Channel]
	if !ok {
		return nil, nil, nil, fmt.Errorf("No channel registered for %s", recvMsg.Channel)
	}

	result := make(chan string, 100)
	done := make(chan bool, 1)
	channelHandler(recvMsg.Body, result, done)
	return recvMsg, result, done, nil
}

func (s *sockjsVessel) dispatchResponses(id, channel string, c <-chan string,
	done <-chan bool, session sockjs.Session) {

	for {
		select {
		case <-done:
			return
		case result := <-c:
			sendMsg := &message{
				ID:      id,
				Channel: channel,
				Body:    result,
			}

			if send, err := s.marshaler.Marshal(sendMsg); err != nil {
				log.Println(err)
			} else {
				sendStr := string(send)
				log.Println("Send", sendStr)
				session.Send(sendStr)
			}
		}
	}
}
