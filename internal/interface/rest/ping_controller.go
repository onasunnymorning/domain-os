package rest

import (
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/gin-gonic/gin"
)

// PingController is a controller for the ping endpoint
type PingController struct {
}

// NewPingController creates a new ping controller
func NewPingController(e *gin.Engine) *PingController {
	controller := &PingController{}

	e.GET("/ping", controller.Ping)

	return controller
}

// Ping is the handler for the ping endpoint
func (ctrl *PingController) Ping(ctx *gin.Context) {
	producer, exists := ctx.Get("kafkaProducer")
	if !exists {
		ctx.JSON(500, gin.H{"message": "Kafka producer not found"})
		return
	}
	topic, exists := ctx.Get("kafkaTopic")
	if !exists {
		ctx.JSON(501, gin.H{"message": "Kafka topic not found"})
		return
	}
	// Assert the type of the producer and topic
	kafkaProducer := producer.(*kafka.Producer)
	kafkatopic := topic.(string)

	err := kafkaProducer.Produce(
		&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &kafkatopic, Partition: kafka.PartitionAny},
			Value:          []byte("ping"),
		},
		nil,
	)
	if err != nil {
		ctx.JSON(500, gin.H{"message": "error producing message to topic ping"})
		return
	}
	ctx.JSON(200, gin.H{"message": "pong"})
}
