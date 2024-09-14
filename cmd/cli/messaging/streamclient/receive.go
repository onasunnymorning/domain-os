package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
)

func CheckErrReceive(err error) {
	if err != nil {
		fmt.Printf("%s ", err)
		os.Exit(1)
	}
}
func main() {

	portString := os.Getenv("RMQ_PORT")
	portInt, err := strconv.Atoi(portString)
	CheckErrReceive(errors.Join(errors.New("failed to convert port to int"), err))

	env, err := stream.NewEnvironment(
		stream.NewEnvironmentOptions().
			SetHost(os.Getenv("RMQ_HOST")).
			SetPort(portInt).
			SetUser(os.Getenv("RMQ_USER")).
			SetPassword(os.Getenv("RMQ_PASS")))
	CheckErrReceive(errors.Join(errors.New("failed to create stream environment"), err))

	streamName := os.Getenv("EVENT_STREAM_TOPIC")
	err = env.DeclareStream(streamName,
		&stream.StreamOptions{
			MaxLengthBytes: stream.ByteCapacity{}.GB(2),
		},
	)
	CheckErrReceive(errors.Join(errors.New("failed to declare stream"), err))

	messagesHandler := func(consumerContext stream.ConsumerContext, message *amqp.Message) {
		fmt.Printf("Stream: %s - Received message: %s\n", consumerContext.Consumer.GetStreamName(),
			message.Data)
	}

	consumer, err := env.NewConsumer(streamName, messagesHandler,
		stream.NewConsumerOptions().SetOffset(stream.OffsetSpecification{}.First()))
	CheckErrReceive(errors.Join(errors.New("failed to create consumer"), err))

	fmt.Println(" [x] Waiting for messages.")

	// Optional: Add logic to gracefully close the consumer and environment on shutdown
	defer func() {
		err = consumer.Close()
		CheckErrReceive(errors.Join(errors.New("failed to close consumer"), err))
		err = env.Close()
		CheckErrReceive(errors.Join(errors.New("failed to close environment"), err))
	}()

	// Keep the program running indefinitely
	for {
		time.Sleep(time.Second)
	}

}
