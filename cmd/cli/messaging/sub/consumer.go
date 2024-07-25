package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func main() {
	bootstrapServers := os.Getenv("KAFKA_HOST")
	group := os.Getenv("KAFKA_GROUP")
	topics := []string{os.Getenv("KAFKA_TOPIC")}

	securityProtocol := os.Getenv("KAFKA_SECURITY_PROTOCOL")
	saslMechanism := os.Getenv("KAFKA_SASL_MECHANISM")
	saslUsername := os.Getenv("KAFKA_SASL_USERNAME")
	saslPassword := os.Getenv("KAFKA_SASL_PASSWORD")

	if bootstrapServers == "" || group == "" || len(topics) == 0 || topics[0] == "" {
		fmt.Fprintf(os.Stderr, "Error: Missing required environment variables\n")
		os.Exit(1)
	}

	// echo all the envars
	fmt.Printf("KAFKA_HOST: %s\n", bootstrapServers)
	fmt.Printf("KAFKA_GROUP: %s\n", group)
	fmt.Printf("KAFKA_TOPIC: %s\n", topics[0])
	fmt.Printf("KAFKA_SECURITY_PROTOCOL: %s\n", securityProtocol)
	fmt.Printf("KAFKA_SASL_MECHANISM: %s\n", saslMechanism)
	fmt.Printf("KAFKA_SASL_USERNAME: %s\n", saslUsername)
	fmt.Printf("KAFKA_SASL_PASSWORD: %s\n", saslPassword)

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":        bootstrapServers,
		"broker.address.family":    "v4",
		"group.id":                 group,
		"session.timeout.ms":       6000,
		"auto.offset.reset":        "earliest",
		"enable.auto.offset.store": false,
		"security.protocol":        securityProtocol,
		"sasl.mechanism":           saslMechanism,
		"sasl.username":            saslUsername,
		"sasl.password":            saslPassword,
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create consumer: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created Consumer %v\n", c)

	err = c.SubscribeTopics(topics, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to subscribe to topics: %s\n", err)
		os.Exit(1)
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	run := true
	for run {
		select {
		case sig := <-sigchan:
			fmt.Printf("Caught signal %v: terminating\n", sig)
			run = false
		default:
			ev := c.Poll(100)
			if ev == nil {
				continue
			}

			switch e := ev.(type) {
			case *kafka.Message:
				fmt.Printf("%% Message on %s:\n%s\n", e.TopicPartition, string(e.Value))
				if e.Headers != nil {
					fmt.Printf("%% Headers: %v\n", e.Headers)
				}

				_, err := c.StoreMessage(e)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%% Error storing offset after message %s:\n", e.TopicPartition)
				}
			case kafka.Error:
				fmt.Fprintf(os.Stderr, "%% Error: %v: %v\n", e.Code(), e)
				if e.Code() == kafka.ErrAllBrokersDown {
					run = false
				}
			default:
				fmt.Printf("Ignored %v\n", e)
			}
		}
	}

	fmt.Printf("Closing consumer\n")
	c.Close()
}
