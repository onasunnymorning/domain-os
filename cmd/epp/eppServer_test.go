package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"net"
	"testing"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockWriter implements the epp.Writer interface for testing
type MockWriter struct {
	buffer          bytes.Buffer
	shouldClose     bool
	closeAfterWrite bool
}

func (m *MockWriter) Write(p []byte) (int, error) {
	return m.buffer.Write(p)
}

func (m *MockWriter) GetWrittenXML() string {
	return m.buffer.String()
}

func (m *MockWriter) CloseAfterWrite() {
	m.closeAfterWrite = true
}

func (m *MockWriter) Reset() {
	m.buffer.Reset()
	m.closeAfterWrite = false
}

func (m *MockWriter) ShouldCloseAfterWrite() bool {
	return m.closeAfterWrite
}

// TestSendGreeting verifies that the greeting XML is properly formatted
func TestSendGreeting(t *testing.T) {
	ctx := context.Background()
	mockWriter := &MockWriter{}

	sendGreeting(ctx, mockWriter, nil)

	// Parse the written XML
	doc := etree.NewDocument()
	err := doc.ReadFromString(mockWriter.GetWrittenXML())
	require.NoError(t, err, "Greeting XML should be valid")

	// Verify structure according to EPP RFC 5730
	greeting := doc.FindElement("//greeting")
	require.NotNil(t, greeting, "Should have greeting element")

	// Check required elements
	svID := greeting.FindElement("svID")
	assert.NotNil(t, svID, "Should have svID element")
	assert.NotEmpty(t, svID.Text(), "svID should not be empty")

	svDate := greeting.FindElement("svDate")
	assert.NotNil(t, svDate, "Should have svDate element")
	assert.NotEmpty(t, svDate.Text(), "svDate should not be empty")

	svcMenu := greeting.FindElement("svcMenu")
	assert.NotNil(t, svcMenu, "Should have svcMenu element")

	version := svcMenu.FindElement("version")
	assert.NotNil(t, version, "Should have version element")
	assert.Equal(t, "1.0", version.Text(), "Should support EPP 1.0")

	lang := svcMenu.FindElement("lang")
	assert.NotNil(t, lang, "Should have lang element")
	assert.Equal(t, "en", lang.Text(), "Should support English")
}

// TestRespondToLoginCommand verifies login response handling
func TestRespondToLoginCommand(t *testing.T) {
	tests := []struct {
		name        string
		inputXML    string
		wantCode    string
		wantMessage string
		expectError bool
	}{
		{
			name: "valid login request",
			inputXML: `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <command>
    <login>
      <clID>testuser</clID>
      <pw>testpass</pw>
      <options>
        <version>1.0</version>
        <lang>en</lang>
      </options>
      <svcs>
        <objURI>urn:ietf:params:xml:ns:domain-1.0</objURI>
        <objURI>urn:ietf:params:xml:ns:contact-1.0</objURI>
        <objURI>urn:ietf:params:xml:ns:host-1.0</objURI>
      </svcs>
    </login>
    <clTRID>ABC-12345</clTRID>
  </command>
</epp>`,
			wantCode:    "1000",
			wantMessage: "Command completed successfully",
		},
		{
			name: "login with minimal data",
			inputXML: `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <command>
    <login>
      <clID>user2</clID>
      <pw>pass2</pw>
    </login>
  </command>
</epp>`,
			wantCode:    "1000",
			wantMessage: "Command completed successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockWriter := &MockWriter{}

			doc := etree.NewDocument()
			err := doc.ReadFromString(tt.inputXML)
			require.NoError(t, err, "Input XML should be valid")

			respondToLoginCommand(ctx, mockWriter, doc)

			// Verify response XML
			respDoc := etree.NewDocument()
			err = respDoc.ReadFromString(mockWriter.GetWrittenXML())
			require.NoError(t, err, "Response XML should be valid")

			// Check result code
			result := respDoc.FindElement("//result")
			require.NotNil(t, result, "Should have result element")
			assert.Equal(t, tt.wantCode, result.SelectAttrValue("code", ""), "Result code should match")

			// Check message
			msg := result.FindElement("msg")
			require.NotNil(t, msg, "Should have msg element")
			assert.Contains(t, msg.Text(), tt.wantMessage, "Message should match")

			// Verify transaction ID exists
			trID := respDoc.FindElement("//trID")
			assert.NotNil(t, trID, "Should have transaction ID")
		})
	}
}

// TestRespondToLogoutCommand verifies logout handling
func TestRespondToLogoutCommand(t *testing.T) {
	ctx := context.Background()
	mockWriter := &MockWriter{}

	logoutXML := `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <command>
    <logout/>
    <clTRID>ABC-12345</clTRID>
  </command>
</epp>`

	doc := etree.NewDocument()
	err := doc.ReadFromString(logoutXML)
	require.NoError(t, err)

	respondToLogoutCommand(ctx, mockWriter, doc)

	// Verify response
	respDoc := etree.NewDocument()
	err = respDoc.ReadFromString(mockWriter.GetWrittenXML())
	require.NoError(t, err)

	// Check result code 1500 (command completed successfully; ending session)
	result := respDoc.FindElement("//result")
	require.NotNil(t, result)
	assert.Equal(t, "1500", result.SelectAttrValue("code", ""))

	// Note: CloseAfterWrite behavior is tested in integration tests
	// The mock writer receives CloseAfterWrite() call but the type assertion
	// in the actual code only works with *epp.ResponseWriter
}

// TestRespondToDomainCheckCommand verifies domain check response
func TestRespondToDomainCheckCommand(t *testing.T) {
	ctx := context.Background()
	mockWriter := &MockWriter{}

	checkXML := `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <command>
    <check>
      <domain:check xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
        <domain:name>example.com</domain:name>
        <domain:name>example.net</domain:name>
      </domain:check>
    </check>
    <clTRID>ABC-12345</clTRID>
  </command>
</epp>`

	doc := etree.NewDocument()
	err := doc.ReadFromString(checkXML)
	require.NoError(t, err)

	respondToDomainCheckCommand(ctx, mockWriter, doc)

	// Verify response is valid XML
	respDoc := etree.NewDocument()
	err = respDoc.ReadFromString(mockWriter.GetWrittenXML())
	require.NoError(t, err, "Response should be valid XML")

	// Verify basic structure
	result := respDoc.FindElement("//result")
	assert.NotNil(t, result, "Should have result element")

	// Check for domain check data
	chkData := respDoc.FindElement("//domain:chkData")
	assert.NotNil(t, chkData, "Should have domain check data")
}

// TestGetGreetingXML verifies greeting XML format
func TestGetGreetingXML(t *testing.T) {
	xml := getGreetingXML()

	doc := etree.NewDocument()
	err := doc.ReadFromString(xml)
	require.NoError(t, err, "Greeting XML should be valid")

	// Verify EPP namespace
	root := doc.Root()
	assert.Equal(t, "epp", root.Tag, "Root element should be epp")
	assert.Equal(t, "urn:ietf:params:xml:ns:epp-1.0", root.SelectAttrValue("xmlns", ""))
}

// TestGetLoginResponseXML verifies login response format
func TestGetLoginResponseXML(t *testing.T) {
	xml := getLoginResponseXML()

	doc := etree.NewDocument()
	err := doc.ReadFromString(xml)
	require.NoError(t, err, "Login response XML should be valid")

	// Verify structure
	result := doc.FindElement("//result")
	require.NotNil(t, result)
	assert.Equal(t, "1000", result.SelectAttrValue("code", ""))
}

// TestGetLogoutResponseXML verifies logout response format
func TestGetLogoutResponseXML(t *testing.T) {
	xml := getLogoutResponseXML()

	doc := etree.NewDocument()
	err := doc.ReadFromString(xml)
	require.NoError(t, err, "Logout response XML should be valid")

	// Verify structure
	result := doc.FindElement("//result")
	require.NotNil(t, result)
	assert.Equal(t, "1500", result.SelectAttrValue("code", ""), "Logout should use code 1500")
}

// TestGenerateCertificate verifies certificate generation
func TestGenerateCertificate(t *testing.T) {
	cert := generateCertificate()

	assert.NotNil(t, cert.Certificate, "Certificate should be generated")
	assert.NotEmpty(t, cert.Certificate, "Certificate should not be empty")
	assert.NotNil(t, cert.PrivateKey, "Private key should be generated")
}

// TestLogConnection verifies connection context management
func TestLogConnection(t *testing.T) {
	ctx := context.Background()

	// Create a mock TCP connection for testing
	// We'll use a simple net.Pipe to create a connection
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// We need a TLS connection, so let's create a minimal TLS config
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{generateCertificate()},
	}

	// Create TLS connection from the client side
	tlsConn := tls.Client(client, tlsConfig)

	newCtx, err := logConnection(ctx, tlsConn)

	require.NoError(t, err, "Should not return error")
	require.NotNil(t, newCtx, "Should return context")

	// Verify context has connection ID
	connID, ok := appcontext.ConnectionID(newCtx)
	assert.True(t, ok, "Context should have connection ID")
	assert.NotEmpty(t, connID, "Connection ID should not be empty")

	// Verify context has client IP
	clientIP, ok := appcontext.ClientIP(newCtx)
	assert.True(t, ok, "Context should have client IP")
	assert.NotEmpty(t, clientIP, "Client IP should not be empty")
}
