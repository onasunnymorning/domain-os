package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dotse/epp-client/pkg"
	"github.com/gin-gonic/gin"
)

var (
	PORT      = "700"
	HOST      = "epp-ote.centralnic.com"
	POOL_SIZE = 2 // Adjust the pool size as needed
)

type ClientPool struct {
	mu      sync.Mutex
	clients chan *pkg.Client
	host    string
	port    string
}

func NewClientPool(ctx context.Context, host, port string, poolSize int) (*ClientPool, error) {
	pool := &ClientPool{
		clients: make(chan *pkg.Client, poolSize),
		host:    host,
		port:    port,
	}

	for i := 0; i < poolSize; i++ {
		client, err := pool.connectAndLogin(ctx)
		if err != nil {
			return nil, err
		}
		pool.clients <- client
	}

	return pool, nil
}

func (p *ClientPool) connectAndLogin(ctx context.Context) (*pkg.Client, error) {
	log.Println("Connecting to EPP server...")
	cl, err := pkg.Connect(net.JoinHostPort(p.host, p.port), &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // should only be used for testing..
		Certificates:       []tls.Certificate{generateCertificate()},
	})
	if err != nil {
		log.Printf("Failed to connect to EPP server: %v", err)
		return nil, err
	}

	log.Println("Connected to EPP server successfully")

	cl.KeepAlive(ctx)

	log.Println("Keepalive activated")

	// Create a login data object
	loginData := &pkg.LoginData{
		Username: "H1056502248-OTE",
		Password: "m8u5:}PKy[C1}dBJ",
		Namespaces: []string{
			"urn:ietf:params:xml:ns:host-1.0",
			"urn:ietf:params:xml:ns:contact-1.0",
			"urn:ietf:params:xml:ns:domain-1.0",
		},
		ExtensionNamespaces: []string{},
		Version:             "1.0",
		Lang:                "en",
		ClTrID:              "ABC-12345",
	}

	// Send the login command
	loginResponse, err := cl.SendCommandUsingTemplate("login.xml", loginData)
	if err != nil {
		return nil, err
	}
	fmt.Println(loginResponse)

	return cl, nil
}

func (p *ClientPool) Get(ctx context.Context) (*pkg.Client, error) {
	log.Println("Trying to get a client from the pool...")
	select {
	case client := <-p.clients:
		log.Println("Got a client from the pool")
		return client, nil
	default:
		log.Println("Pool is empty, creating a new client...")
		return p.connectAndLogin(ctx)
	}
}

func (p *ClientPool) Release(client *pkg.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Println("Releasing client back to the pool...")
	select {
	case p.clients <- client:
		log.Println("Client released successfully")
	default:
		log.Println("Pool is full, closing the client...")
		client.SendCommandUsingTemplate("logout.xml", nil) // Close the connection if the pool is full
	}
}

func (p *ClientPool) KeepAlive(ctx context.Context, errorChan chan<- error) {
	log.Println("Starting client pool keep-alive mechanism...")
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping client pool keep-alive mechanism...")
			return
		default:
			// Check if the number of clients in the pool is less than POOL_SIZE
			if len(p.clients) < POOL_SIZE {
				// Create a new client and add it to the pool
				client, err := p.connectAndLogin(ctx)
				if err != nil {
					log.Printf("Failed to connect and login: %v", err)
					continue // Skip to the next iteration
				}
				p.clients <- client
			}
			// Sleep for a short duration to prevent busy looping
			time.Sleep(1 * time.Second)
		}
	}
}

func main() {
	ctx := context.Background()

	clientPool, err := NewClientPool(ctx, HOST, PORT, POOL_SIZE)
	if err != nil {
		log.Fatalf("failed to create client pool: %v", err)
	}

	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		log.Println("Received ping request")
		c.String(http.StatusOK, "pong")
	})

	r.GET("/hello", func(c *gin.Context) {
		log.Println("Received hello request")
		client, err := clientPool.Get(ctx)
		if err != nil {
			log.Printf("Failed to get client from pool: %v", err)
			c.String(http.StatusInternalServerError, fmt.Sprintf("error: %s", err.Error()))
			return
		}
		defer clientPool.Release(client)

		helloResponse, err := client.SendCommandUsingTemplate("hello.xml", nil)
		if err != nil {
			log.Printf("Failed to send hello command: %v", err)
			c.String(http.StatusInternalServerError, fmt.Sprintf("error: %s", err.Error()))
			return
		}
		// Set content type to XML and send the response
		c.Data(http.StatusOK, "application/xml", []byte(helloResponse))
	})

	r.GET("/login", func(c *gin.Context) {
		log.Println("Received login request")
		client, err := clientPool.Get(ctx)
		if err != nil {
			log.Printf("Failed to get client from pool: %v", err)
			c.String(http.StatusInternalServerError, fmt.Sprintf("error: %s", err.Error()))
			return
		}
		defer clientPool.Release(client)

		// Create a login data object
		loginData := &pkg.LoginData{
			Username:            "H1056502248-OTE",
			Password:            "m8u5:}PKy[C1}dBJ",
			Namespaces:          []string{},
			ExtensionNamespaces: []string{},
			Version:             "1.0",
			Lang:                "en",
			ClTrID:              "ABC-12345",
		}

		// Send the login command
		loginResponse, err := client.SendCommandUsingTemplate("login.xml", loginData)
		if err != nil {
			log.Printf("Failed to send login command: %v", err)
			c.String(http.StatusInternalServerError, fmt.Sprintf("error: %s", err.Error()))
			return
		}
		// Set content type to XML and send the response
		c.Data(http.StatusOK, "application/xml", []byte(loginResponse))
	})

	r.GET("/domain/check", func(c *gin.Context) {
		log.Println("Received domain check request")
		params := c.QueryArray("domain")

		client, err := clientPool.Get(ctx)
		if err != nil {
			log.Printf("Failed to get client from pool: %v", err)
			c.String(http.StatusInternalServerError, fmt.Sprintf("error: %s", err.Error()))
			return
		}
		defer clientPool.Release(client)

		// Create a domain check data object
		domainCheckData := &pkg.DomainCheck{
			DomainNames: params,
			ClTrID:      "ABC-123",
		}

		// Send the domain check command
		domainCheckResponse, err := client.SendCommandUsingTemplate("domain_check.xml", domainCheckData)
		if err != nil {
			log.Printf("Failed to send domain check command: %v", err)
			c.String(http.StatusInternalServerError, fmt.Sprintf("error: %s", err.Error()))
			return
		}
		// Set content type to XML and send the response
		c.Data(http.StatusOK, "application/xml", []byte(domainCheckResponse))
	})

	// RAW
	// Raw endpoint accepts XML POST and sends it to the EPP server, and returns the  XML response
	r.POST("/raw", func(c *gin.Context) {
		log.Println("Received raw request")
		client, err := clientPool.Get(ctx)
		if err != nil {
			log.Printf("Failed to get client from pool: %v", err)
			c.String(http.StatusInternalServerError, fmt.Sprintf("error: %s", err.Error()))
			return
		}
		defer clientPool.Release(client)

		// Read the XML request from the body
		xmlRequest, err := c.GetRawData()
		if err != nil {
			log.Printf("Failed to read request body: %v", err)
			c.String(http.StatusInternalServerError, fmt.Sprintf("error: %s", err.Error()))
			return
		}

		log.Printf("Received XML:\n%s", string(xmlRequest))

		// Send the XML request to the EPP server
		xmlResponse, err := client.SendCommand(xmlRequest)
		if err != nil {
			log.Printf("Failed to send command: %v", err)
			c.String(http.StatusInternalServerError, fmt.Sprintf("error: %s", err.Error()))
			return
		}

		// Log
		log.Printf("Returned XML:\n%s", string(xmlResponse))

		// Set content type to XML and send the response
		c.Data(http.StatusOK, "application/xml", []byte(xmlResponse))
	})

	// Start the gin server
	if err := r.Run(":8700"); err != nil {
		log.Fatalf("failed to start gin server: %v", err)
	}
}

// generateCertificate generates a self-signed certificate in case client side certificates are not provided.
func generateCertificate() tls.Certificate {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		Subject: pkix.Name{
			CommonName:   "epp.example.test",
			Organization: []string{"Simple Server Test"},
			Country:      []string{"SE"},
			Locality:     []string{"Stockholm"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, 1),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	certificate, _ := x509.CreateCertificate(rand.Reader, cert, cert, key.Public(), key)

	return tls.Certificate{
		Certificate: [][]byte{certificate},
		PrivateKey:  key,
	}
}
