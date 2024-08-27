package main

import (
	"fmt"
	"log"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	ExpiryQueueTopic = "expiry-domains-queue"
	RMQ_HOST         = "localhost"
	RMQ_PORT         = "5672"
	RMQ_USER         = os.Getenv("RMQ_USER")
	RMQ_PASS         = os.Getenv("RMQ_PASS")
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func main() {
	msgUrl := fmt.Sprintf("amqp://%s:%s@%s:%s/", RMQ_USER, RMQ_PASS, RMQ_HOST, RMQ_PORT)
	fmt.Println(msgUrl)
	conn, err := amqp.Dial(fmt.Sprintf(msgUrl))
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		ExpiryQueueTopic, // name
		false,            // durable
		false,            // delete when unused
		false,            // exclusive
		false,            // no-wait
		nil,              // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	var forever chan struct{}

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}
