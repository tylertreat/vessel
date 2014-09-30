package vessel

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/mattrobenolt/gocql/uuid"
)

// Channel is a function which takes a message, a channel for sending results,
// and a channel for signaling that the handler has completed.
type Channel func(string, chan<- string, chan<- bool)

// Vessel coordinates communication between server and peers. It's responsible
// for managing Channels and processing incoming and outgoing messages.
type Vessel interface {
	// AddChannel registers the Channel handler with the specified name.
	AddChannel(string, Channel)

	// Start will start the server with the given port configuration. Once
	// started, the Vessel will begin dispatching messages.
	Start(map[string]string) error

	// Publish sends the specified message on the given channel to all
	// connected peers.
	Publish(string, string)

	// Transports returns a slice containing the names of all registered
	// message transports.
	Transports() []string
}

type vessel struct {
	transports       map[string]transport
	channels         map[string]Channel
	marshaler        marshaler
	idGenerator      idGenerator
	messageGenerator messageGenerator
	persister        Persister
	messages         <-chan *messageContext
}

// NewVessel creates and returns a new Vessel.
func NewVessel(uri string) Vessel {
	messages := make(chan *messageContext, 10000)
	persister := NewPersister()
	v := &vessel{
		channels:         map[string]Channel{},
		marshaler:        &jsonMarshaler{},
		idGenerator:      newUUID,
		messageGenerator: newMessage,
		persister:        persister,
		messages:         messages,
		transports: map[string]transport{
			"sockjs": newSockJSTransport(uri, messages),
			"http":   newHTTPTransport(uri, messages, persister),
		},
	}
	return v
}

type transport interface {
	listenAndServe(string) error
	publish(*message)
}

type responseDispatcher func(string, string, <-chan string, <-chan bool)

type Persister interface {
	Prepare() error
	SaveResult(string, *result) error
	SaveMessage(string, *message) error
	GetResult(string) (*result, error)
	GetMessages(string, int64) ([]*message, error)
}

type message struct {
	ID        string `json:"id"`
	Channel   string `json:"channel"`
	Body      string `json:"body"`
	Timestamp int64  `json:"timestamp"`
}

type messageContext struct {
	message    *message
	dispatcher responseDispatcher
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

// AddChannel registers the Channel handler with the specified name.
func (v *vessel) AddChannel(name string, channel Channel) {
	v.channels[name] = channel
}

// Start will start the server with the given port configuration. Once started,
// the Vessel will begin dispatching messages.
func (v *vessel) Start(ports map[string]string) error {
	if err := v.persister.Prepare(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	go v.dispatch()
	for name, transport := range v.transports {
		if port, ok := ports[name]; ok {
			wg.Add(1)
			go transport.listenAndServe(port)
		}
	}
	log.Println("Vessel is started")
	wg.Wait()
	return nil
}

// Transports returns a slice containing the names of all registered message
// transports.
func (v *vessel) Transports() []string {
	transports := make([]string, 0, len(v.transports))
	for transport, _ := range v.transports {
		transports = append(transports, transport)
	}
	return transports
}

// Publish sends the specified message on the given channel to all connected
// peers.
func (v *vessel) Publish(channel, message string) {
	msg := v.messageGenerator(v.idGenerator(), channel, message)
	v.persister.SaveMessage(channel, msg)
	for _, transport := range v.transports {
		transport.publish(msg)
	}
}

func (v *vessel) dispatch() {
	for {
		v.recv(<-v.messages)
	}
}

func (v *vessel) recv(msgCtx *messageContext) {
	msg := msgCtx.message
	log.Printf("Recv %s:%s:%s", msg.ID, msg.Channel, msg.Body)

	channelHandler, ok := v.channels[msg.Channel]
	if !ok {
		log.Printf("No channel registeted for %s", msg.Channel)
		return
	}

	result := make(chan string, 1)
	done := make(chan bool, 1)
	go channelHandler(msg.Body, result, done)
	go msgCtx.dispatcher(msg.ID, msg.Channel, result, done)
}
