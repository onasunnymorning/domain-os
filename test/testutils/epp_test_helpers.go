package testutils

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	epp "gitlab.com/internetstiftelsen-oss/epp-lib"
)

// TestEPPServer wraps an EPP server for testing purposes
type TestEPPServer struct {
	Server   *epp.Server
	Listener net.Listener
	Port     int
	stopChan chan struct{}
}

// NewTestEPPServer creates a new test EPP server on a random available port
func NewTestEPPServer(commandMux *epp.CommandMux, tlsConfig tls.Config) (*TestEPPServer, error) {
	// Listen on random available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	server := &epp.Server{
		HandleCommand:  commandMux.Handle,
		Greeting:       commandMux.GetGreeting,
		TLSConfig:      tlsConfig,
		Timeout:        30 * time.Second,
		IdleTimeout:    10 * time.Second,
		WriteTimeout:   5 * time.Second,
		ReadTimeout:    5 * time.Second,
		MaxMessageSize: 10000,
	}

	return &TestEPPServer{
		Server:   server,
		Listener: listener,
		Port:     port,
		stopChan: make(chan struct{}),
	}, nil
}

// Start starts the test server in a goroutine
func (s *TestEPPServer) Start() error {
	go func() {
		s.Server.Serve(s.Listener)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Stop stops the test server
func (s *TestEPPServer) Stop() error {
	close(s.stopChan)
	return s.Listener.Close()
}

// GetAddress returns the server address
func (s *TestEPPServer) GetAddress() string {
	return fmt.Sprintf("localhost:%d", s.Port)
}

// MockWriter implements the epp.Writer interface for testing
type MockWriter struct {
	buffer          bytes.Buffer
	closeAfterWrite bool
}

// NewMockWriter creates a new mock writer
func NewMockWriter() *MockWriter {
	return &MockWriter{}
}

// Write writes data to the buffer
func (m *MockWriter) Write(p []byte) (int, error) {
	return m.buffer.Write(p)
}

// GetWrittenXML returns the written XML as a string
func (m *MockWriter) GetWrittenXML() string {
	return m.buffer.String()
}

// CloseAfterWrite marks the connection to be closed
func (m *MockWriter) CloseAfterWrite() {
	m.closeAfterWrite = true
}

// ShouldCloseAfterWrite returns whether the connection should be closed
func (m *MockWriter) ShouldCloseAfterWrite() bool {
	return m.closeAfterWrite
}

// Reset clears the buffer and flags
func (m *MockWriter) Reset() {
	m.buffer.Reset()
	m.closeAfterWrite = false
}
