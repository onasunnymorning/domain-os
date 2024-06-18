package kafkaproducer

import "github.com/confluentinc/confluent-kafka-go/v2/kafka"

// InitEventProducer creates a new event producer
func InitEventProducer(bootstrapServers string) (*kafka.Producer, error) {
	return kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": bootstrapServers})
}
