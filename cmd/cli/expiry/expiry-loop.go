package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	amqp "github.com/rabbitmq/amqp091-go"
)

// This is aimed to run continuously to check for domains that are about to expire and send them to the expiry queue

const ()

var (
	RMQ_HOST        = "localhost"
	RMQ_PORT        = "5672"
	RMQ_USER        = os.Getenv("RMQ_USER")
	RMQ_PASS        = os.Getenv("RMQ_PASS")
	ExpiryQueueName = os.Getenv("EXPIRY_QUEUE_NAME")
	BATCHSIZE       = 25

	API_HOST       = os.Getenv("API_HOST")
	API_PORT       = os.Getenv("API_PORT")
	BASE_URL       = fmt.Sprintf("http://%s:%s", API_HOST, API_PORT)
	LIST_ENDPOINT  = BASE_URL + "/domains/expiring?days=1"
	COUNT_ENDPOINT = BASE_URL + "/domains/expiring/count?days=1"
)

// This scripts pulls a list of domains that are about to expire and sends them to the expiry queue

func main() {

	// Set up a reusable API client
	client := http.Client{}

	// check the total amount of domains to renew
	req, err := http.NewRequest("GET", COUNT_ENDPOINT, nil)
	if err != nil {
		panic(err)
	}
	resp, body, err := doRequest(&client, req)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to fetch domain count (%d): %s", resp.StatusCode, body)
		panic("Failed to fetch domain count")
	}
	// Parse the result
	countResponse := &response.CountResult{}
	err = json.Unmarshal(body, &countResponse)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Total domains to renew: %d\n", countResponse.Count)

	// Pull in one batch of domains
	fmt.Printf("Fetching domains in batches of %d\n", BATCHSIZE)
	req, err = http.NewRequest("GET", LIST_ENDPOINT, nil)
	if err != nil {
		panic(err)
	}
	resp, body, err = doRequest(&client, req)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to fetch domains (%d): %s", resp.StatusCode, body)
		panic("Failed to fetch domains")
	}
	// Parse the result
	apiResponse := &ListItemResult{}
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Got %d domains to renew in this batch\n", len(apiResponse.Data))

	// Create Queue producer
	fmt.Println("Queueing domain expiry jobs")
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%s/", RMQ_USER, RMQ_PASS, RMQ_HOST, RMQ_PORT))
	if err != nil {
		panic(err)
	}
	ch, err := conn.Channel()
	if err != nil {
		panic(err)
	}
	defer ch.Close()
	q, err := ch.QueueDeclare(
		ExpiryQueueName, // name
		false,           // durable
		false,           // delete when unused
		false,           // exclusive
		false,           // no-wait
		nil,             // arguments
	)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Send the list of domains to the expiry queue
	for _, domain := range apiResponse.Data {
		jsonDomain, err := json.Marshal(domain)
		if err != nil {
			panic(err)
		}
		err = ch.PublishWithContext(
			ctx,
			"",     // exchange
			q.Name, // routing key
			false,  // mandatory
			false,  // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(jsonDomain),
			})
		if err != nil {
			panic(err)
		}
		// sleep 1 sec
		// time.Sleep(1 * time.Second)
	}

}

type DomainExpiryItem struct {
	RoID       string `json:"ro_id"`
	Name       string `json:"name"`
	ExpiryDate string `json:"expiry_date"`
}

type MetaData struct {
	Cursor     string `json:"cursor"`
	PageSize   int    `json:"pageSize"`
	PageCursor string `json:"pageCursor"`
	NextLink   string `json:"nextLink"`
}

type ListItemResult struct {
	Data []DomainExpiryItem `json:"data"`
	Meta MetaData           `json:"meta"`
}

func doRequest(client *http.Client, req *http.Request) (*http.Response, []byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}
	return resp, body, nil
}
