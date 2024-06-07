package rest

import (
	"log"

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
			Value:          event.ToBytes(),
		},
		nil,
	)

}
