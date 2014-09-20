package vessel

import (
	"fmt"
	"log"
	"net/http"

	"github.com/igm/sockjs-go/sockjs"
)

type sockjsVessel struct {
	uri         string
	sessions    []sockjs.Session
	channels    map[string]Channel
	marshaler   marshaler
	idGenerator idGenerator
}

func NewSockJSVessel(uri string) Vessel {
	return &sockjsVessel{
		uri:         uri,
		channels:    map[string]Channel{},
		sessions:    []sockjs.Session{},
		marshaler:   &jsonMarshaler{},
		idGenerator: &uuidGenerator{},
	}
}

func (v *sockjsVessel) AddChannel(name string, channel Channel) {
	v.channels[name] = channel
}

func (v *sockjsVessel) Start(portStr string) error {
	handler := sockjs.NewHandler(v.uri, sockjs.DefaultOptions, v.handler())
	return http.ListenAndServe(portStr, handler)
}

func (s *sockjsVessel) Broadcast(channel string, msg string) {
	m := &message{
		ID:      s.idGenerator.generate(),
		Channel: channel,
		Body:    msg,
	}

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

			log.Println("Recv", msg)

			recvMsg, err := s.marshaler.unmarshal([]byte(msg))
			if err != nil {
				log.Println(err)
				continue
			}

			channelHandler, ok := s.channels[recvMsg.Channel]
			if !ok {
				log.Println(fmt.Sprintf("No channel registered for %s", recvMsg.Channel))
				continue
			}

			result := make(chan string)
			done := make(chan bool)
			go s.listenForResults(recvMsg.ID, recvMsg.Channel, result, done, session)
			channelHandler(recvMsg.Body, result, done)
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

func (s *sockjsVessel) listenForResults(id, channel string, c <-chan string, done <-chan bool, session sockjs.Session) {
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
