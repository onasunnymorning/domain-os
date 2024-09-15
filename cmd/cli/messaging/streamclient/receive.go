package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
)

func main() {

	portString := os.Getenv("RMQ_PORT")
	portInt, err := strconv.Atoi(portString)
	if err != nil {
		fmt.Printf("failed to convert port to int: %s", err)
		os.Exit(1)
	}

	env, err := stream.NewEnvironment(
		stream.NewEnvironmentOptions().
			SetHost(os.Getenv("RMQ_HOST")).
			SetPort(portInt).
			SetUser(os.Getenv("RMQ_USER")).
			SetPassword(os.Getenv("RMQ_PASS")))
	if err != nil {
		fmt.Printf("failed to create stream environment: %s", err)
		os.Exit(1)
	}

	streamName := os.Getenv("EVENT_STREAM_TOPIC")
	err = env.DeclareStream(streamName,
		&stream.StreamOptions{
			MaxLengthBytes: stream.ByteCapacity{}.GB(2),
		},
	)
	if err != nil {
		fmt.Printf("failed to declare stream: %s", err)
		os.Exit(1)
	}

	messagesHandler := func(consumerContext stream.ConsumerContext, message *amqp.Message) {
		fmt.Printf("Stream: %s - Received message: %s\n", consumerContext.Consumer.GetStreamName(),
			message.Data)
	}

	consumer, err := env.NewConsumer(streamName, messagesHandler,
		stream.NewConsumerOptions().SetOffset(stream.OffsetSpecification{}.First()))
	if err != nil {
		fmt.Printf("failed to create consumer: %s", err)
		os.Exit(1)
	}

	fmt.Println(" [x] Waiting for messages.")

	// Optional: Add logic to gracefully close the consumer and environment on shutdown
	defer func() {
		err = consumer.Close()
		if err != nil {
			fmt.Printf("failed to close consumer: %s", err)
		}
		err = env.Close()
		if err != nil {
			fmt.Printf("failed to close environment: %s", err)
		}
	}()

	// Keep the program running indefinitely
	for {
		time.Sleep(time.Second)
	}

}
