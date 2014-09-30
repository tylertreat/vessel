package vessel

import (
	"log"
	"net/http"

	"github.com/igm/sockjs-go/sockjs"
)

type sockjsTransport struct {
	uri       string
	sessions  []sockjs.Session
	channels  map[string]Channel
	marshaler marshaler
	messages  chan<- *messageContext
}

// newSockJSTransport returns a new transport which relies on SockJS as the
// underlying transport protocol.
func newSockJSTransport(uri string, messages chan<- *messageContext) transport {
	transport := &sockjsTransport{
		uri:       uri,
		channels:  map[string]Channel{},
		sessions:  []sockjs.Session{},
		marshaler: &jsonMarshaler{},
		messages:  messages,
	}
	return transport
}

// Start will start the server on the given port.
func (v *sockjsTransport) listenAndServe(portStr string) error {
	sockjsHandler := sockjs.NewHandler(v.uri, sockjs.DefaultOptions, v.handler())
	return http.ListenAndServe(portStr, sockjsHandler)
}

// Publish sends the specified message on the given channel to all connected
// clients.
func (s *sockjsTransport) publish(msg *message) {
	for _, session := range s.sessions {
		if send, err := s.marshaler.marshal(msg); err != nil {
			log.Println(err)
		} else {
			sendStr := string(send)
			log.Println("Send", sendStr)
			session.Send(sendStr)
		}
	}
}

func (s *sockjsTransport) handler() func(sockjs.Session) {
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

			msgCtx := &messageContext{recvMsg, func(id, channel string, c <-chan string, done <-chan bool) {
				s.dispatchResponses(id, channel, c, done, session)
			}}

			// Ship out the message for processing.
			s.messages <- msgCtx
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

func (s *sockjsTransport) dispatchResponses(id, channel string, c <-chan string,
	done <-chan bool, session sockjs.Session) {

	for {
		select {
		case <-done:
			return
		case result := <-c:
			sendMsg := newMessage(id, channel, result)
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
