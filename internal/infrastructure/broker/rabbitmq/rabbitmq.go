package rabbitmq

import (
	"encoding/json"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
)

type RabbitConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Topic    string
}

// EventRepository implements the EventRepository interface
type EventRepository struct {
	config         *RabbitConfig
	streamProducer *stream.Producer
}

// NewEventRepository creates a new EventRepository
func NewEventRepository(rc *RabbitConfig) (*EventRepository, error) {
	// Create the environment
	env, err := stream.NewEnvironment(
		stream.NewEnvironmentOptions().
			SetHost(rc.Host).
			SetPort(rc.Port).
			SetUser(rc.Username).
			SetPassword(rc.Password))
	if err != nil {
		return nil, err
	}

	// INIT: Declare the stream (this is idempotent so we can do it from the client and server)
	err = env.DeclareStream(rc.Topic,
		&stream.StreamOptions{
			MaxLengthBytes: stream.ByteCapacity{}.GB(2),
		},
	)
	if err != nil {
		return nil, err
	}

	// Create the producer
	sp, err := env.NewProducer(rc.Topic, stream.NewProducerOptions())
	if err != nil {
		return nil, err
	}

	return &EventRepository{
		config:         rc,
		streamProducer: sp,
	}, nil
}

// Send sends an event
func (r *EventRepository) SendStream(event *entities.Event) error {
	// convert the event to a slice of bytes
	e, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// send the event
	return r.streamProducer.Send(amqp.NewMessage(e))
}
