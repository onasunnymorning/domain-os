package rest

import (
	"log"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// logMessage uses the message producer from the context to log a message to the topic
func logMessage(ctx *gin.Context, event *entities.Event) {
	producer, exists := ctx.Get("kafkaProducer")
	if !exists {
		log.Println("Kafka producer not found")
	}
	topic, exists := ctx.Get("kafkaTopic")
	if !exists {
		log.Println("Kafka producer not found")
	}
	// Assert the type of the producer and topic
	kafkaProducer := producer.(*kafka.Producer)
	kafkatopic := topic.(string)

	kafkaProducer.Produce(
		&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &kafkatopic, Partition: kafka.PartitionAny},
			Value:          event.ToJSONBytes(),
		},
		nil,
	)
}

// newEventFromContext creates a new event from the context. It is a wrapper around entities.NewEvent
func newEventFromContext(ctx *gin.Context) *entities.Event {
	return entities.NewEvent(
		ctx.GetString("App"),
		ctx.GetString("userID"),
		getEventTypeFromContext(ctx),
		getObjectTypeFromContext(ctx),
		getObjectIDFromContext(ctx),
		ctx.FullPath(),
	)
}

// getEventTypeFromContext gets the event type from the context
func getEventTypeFromContext(ctx *gin.Context) string {
	url := ctx.FullPath()

	switch strings.Split(url, "/")[1] {
	case "accreditations":
		return entities.EventTypeAccreditation
	}

	return entities.EventTypeUnknown
}

// getObjectTypeFromContext gets the object type from the context
func getObjectTypeFromContext(ctx *gin.Context) string {
	url := ctx.FullPath()

	switch strings.Split(url, "/")[1] {
	case "accreditations":
		return entities.ObjectTypeTLD
	}

	return ""
}

// getObjectIDFromContext gets the object ID from the context
func getObjectIDFromContext(ctx *gin.Context) string {
	url := ctx.FullPath()

	switch strings.Split(url, "/")[1] {
	case "accreditations":
		return ctx.Param("tldName")
	}

	return ""
}
