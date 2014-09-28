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
	marshaler   marshaler
	idGenerator idGenerator
	httpHandler *httpHandler
	persister   Persister
}

// NewSockJSVessel returns a new Vessel which relies on SockJS as the underlying transport.
func NewSockJSVessel(uri string) Vessel {
	vessel := &sockjsVessel{
		uri:         uri,
		channels:    map[string]Channel{},
		sessions:    []sockjs.Session{},
		marshaler:   &jsonMarshaler{},
		idGenerator: &uuidGenerator{},
		persister:   NewPersister(),
	}
	httpHandler := newHTTPHandler(vessel)
	vessel.httpHandler = httpHandler
	return vessel
}

// AddChannel registers the Channel handler with the specified name.
func (v *sockjsVessel) AddChannel(name string, channel Channel) {
	v.channels[name] = channel
}

// Start will start the server on the given port.
func (v *sockjsVessel) Start(sockPortStr, httpPortStr string) error {
	if err := v.persister.Prepare(); err != nil {
		return err
	}

	sockjsHandler := sockjs.NewHandler(v.uri, sockjs.DefaultOptions, v.handler())
	r := mux.NewRouter()
	r.HandleFunc("/_vessel", v.httpHandler.send).Methods("POST")
	r.HandleFunc("/_vessel/message/{id}", v.httpHandler.pollResponses).Methods("GET")
	r.HandleFunc("/_vessel/channel/{channel}", v.httpHandler.pollSubscription).Methods("GET")
	http.Handle("/", &httpServer{r})
	go func() {
		http.ListenAndServe(httpPortStr, nil)
	}()
	return http.ListenAndServe(sockPortStr, sockjsHandler)
}

func (v *sockjsVessel) Persister() Persister {
	return v.persister
}

// Broadcast sends the specified message on the given channel to all connected clients.
func (s *sockjsVessel) Broadcast(channel string, msg string) {
	m := &message{
		ID:      s.idGenerator.generate(),
		Channel: channel,
		Body:    msg,
	}

	s.persister.SaveMessage(channel, m)
	for _, session := range s.sessions {
		if send, err := s.marshaler.marshal(m); err != nil {
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

			recvMsg, err := s.marshaler.unmarshal([]byte(msg))
			if err != nil {
				log.Println(err)
				continue
			}

			// Process message and invoke handler for it.
			results, done, err := s.Recv(recvMsg)
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

// Recv will handle a message by invoking any registered Channel handler. It
// returns channels for receiving responses and checking if the message handler
// has completed.
func (s *sockjsVessel) Recv(msg *message) (<-chan string, <-chan bool, error) {
	log.Printf("Recv %s:%s:%s", msg.ID, msg.Channel, msg.Body)

	channelHandler, ok := s.channels[msg.Channel]
	if !ok {
		return nil, nil, fmt.Errorf("No channel registered for %s", msg.Channel)
	}

	result := make(chan string, 1)
	done := make(chan bool, 1)
	go channelHandler(msg.Body, result, done)
	return result, done, nil
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
			if send, err := s.marshaler.marshal(sendMsg); err != nil {
				log.Println(err)
			} else {
				sendStr := string(send)
				log.Println("Send", sendStr)
				session.Send(sendStr)
			}
		}
	}
}
