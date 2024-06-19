package kafkaproducer

import (
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// InitEventProducer creates a new event producer
func InitEventProducer(bootstrapServers string) (*kafka.Producer, error) {

	eventProducer, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": bootstrapServers})
	if err != nil {
		return nil, fmt.Errorf("Failed to create producer: %v", err)
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
