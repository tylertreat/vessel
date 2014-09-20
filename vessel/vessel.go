package vessel

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mattrobenolt/gocql/uuid"
)

// Channel is a function which takes a message, a channel for sending results, and a channel
// for signaling that the handler has completed.
type Channel func(string, chan<- string, chan<- bool)

// Vessel coordinates communication between clients and server. It's responsible for managing
// Channels and processing incoming and outgoing messages.
type Vessel interface {
	AddChannel(string, Channel)
	Start(string) error
	Broadcast(string, string)
}

type message struct {
	ID      string `json:"id"`
	Channel string `json:"channel"`
	Body    string `json:"body"`
}

type marshaler interface {
	unmarshal([]byte) (*message, error)
	marshal(*message) ([]byte, error)
}

type jsonMarshaler struct{}

func (j *jsonMarshaler) unmarshal(msg []byte) (*message, error) {
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

func (j *jsonMarshaler) marshal(message *message) ([]byte, error) {
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
