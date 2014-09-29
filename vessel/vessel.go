package vessel

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

	// Recv will handle a message by invoking any registered Channel handler.
	// It returns channels for receiving responses and checking if the message
	// handler has completed.
	Recv(*message) (<-chan string, <-chan bool, error)

	// Broadcast sends the specified message on the given channel to all connected clients.
	Broadcast(string, string)

	// Persister returns the Persister for this Vessel.
	Persister() Persister

	// URI returns the registered URI for this Vessel.
	URI() string
}

type Persister interface {
	Prepare() error
	SaveResult(string, *result) error
	SaveMessage(string, *message) error
	GetResult(string) (*result, error)
	GetMessages(string) ([]*message, error)
}

type message struct {
	ID        string `json:"id"`
	Channel   string `json:"channel"`
	Body      string `json:"body"`
	Timestamp int64  `json:"timestamp"`
}

type messageGenerator func(string, string, string) *message

func newMessage(id, channel, body string) *message {
	return &message{
		ID:        id,
		Channel:   channel,
		Body:      body,
		Timestamp: time.Now().Unix(),
	}
}

type idGenerator func() string

func newUUID() string {
	uuid := uuid.RandomUUID().String()
	return strings.Replace(uuid, "-", "", -1)
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

	timestamp, ok := payload["timestamp"]
	if !ok {
		return nil, fmt.Errorf("Message missing timestamp")
	}

	message := &message{
		ID:        id.(string),
		Channel:   channel.(string),
		Body:      body.(string),
		Timestamp: int64(timestamp.(float64)),
	}

	return message, nil
}

func (j *jsonMarshaler) marshal(message *message) ([]byte, error) {
	return json.Marshal(message)
}
