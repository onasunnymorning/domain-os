package kafkaproducer

import (
	"fmt"
	"log"
	"os"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// InitEventProducer creates a new event producer
func InitEventProducer() (*kafka.Producer, error) {
	log.Println("Initializing Kafka producer")

	// Create Kafka producer configuration
	config := &kafka.ConfigMap{
		"bootstrap.servers": os.Getenv("KAFKA_HOST"),
		"security.protocol": os.Getenv("KAFKA_SECURITY_PROTOCOL"),
		"sasl.mechanism":    os.Getenv("KAFKA_SASL_MECHANISM"),
		"sasl.username":     os.Getenv("KAFKA_SASL_USERNAME"),
		"sasl.password":     os.Getenv("KAFKA_SASL_PASSWORD"),
	}

	fmt.Println("Kafka producer config: ", config)

	eventProducer, err := kafka.NewProducer(config)
	if err != nil {
		return nil, err
	}

	// Listen to all the events on the default events channel for errors during message delivery. Since sending is asynchronous, we start this channel to receive the delivery reports in a non-blocking way.
	go func() {
		for e := range eventProducer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				// The message delivery report, indicating success or
				// permanent failure after retries have been exhausted.
				// Application level retries won't help since the client
				// is already configured to do that.
				m := ev
				if m.TopicPartition.Error != nil {
					fmt.Printf("Delivery failed: %v\n", m.TopicPartition.Error)
				} else {
					fmt.Printf("Delivered message to topic %s [%d] at offset %v\n",
						*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)
				}
			case kafka.Error:
				// Generic client instance-level errors, such as
				// broker connection failures, authentication issues, etc.
				//
				// These errors should generally be considered informational
				// as the underlying client will automatically try to
				// recover from any errors encountered, the application
				// does not need to take action on them.
				fmt.Printf("Error: %v\n", ev)
			default:
				fmt.Printf("Ignored event: %s\n", ev)
			}
		}
	}()

	return eventProducer, nil
}
