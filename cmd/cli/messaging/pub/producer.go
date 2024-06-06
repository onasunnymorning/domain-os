package main

import (
	"fmt"
	"log"
	"os"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func main() {

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": "localhost:9092",
		"client.id":         hostname,
		"acks":              "all"})

	if err != nil {
		fmt.Printf("Failed to create producer: %s\n", err)
		os.Exit(1)
	}

	// go func() {
	// 	for e := range p.Events() {
	// 		switch ev := e.(type) {
	// 		case *kafka.Message:
	// 			if ev.TopicPartition.Error != nil {
	// 				fmt.Printf("Failed to deliver message: %v\n", ev.TopicPartition)
	// 			} else {
	// 				fmt.Printf("Successfully produced record to topic %s partition [%d] @ offset %v\n",
	// 					*ev.TopicPartition.Topic, ev.TopicPartition.Partition, ev.TopicPartition.Offset)
	// 			}
	// 		}
	// 	}
	// }()

	topic := "news"
	value := "hello world"

	err = p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          []byte(value)},
		nil, // delivery channel
	)

	if err != nil {
		fmt.Printf("Failed to produce message: %s\n", err)
		os.Exit(1)
	}

	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					fmt.Printf("Failed to deliver message: %v\n", ev.TopicPartition)
				} else {
					fmt.Printf("Successfully produced record to topic %s partition [%d] @ offset %v\n",
						*ev.TopicPartition.Topic, ev.TopicPartition.Partition, ev.TopicPartition.Offset)
				}
			}
		}
	}()

	p.Flush(15 * 1000)

}
