package snowflakeidgenerator

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bwmarrin/snowflake"
)

// IDGenerator implements the IDGenerator interface
type IDGenerator struct {
	node   *snowflake.Node
	nodeID int64
}

// GenerateID generates a new unique ID
func (g *IDGenerator) GenerateID() int64 {
	return g.node.Generate().Int64()
}

// ListNode returns the Node ID of the IDGenerator so we can register it and ensure its uniqueness
func (g *IDGenerator) ListNode() int64 {
	return g.nodeID
}

// NewIDGenerator creates a new instance of IDGenerator
func NewIDGenerator() (*IDGenerator, error) {
	node, err := NewSnowflakeNode()
	if err != nil {
		return nil, err
	}
	return &IDGenerator{
		node:   node,
		nodeID: node.Generate().Node(),
	}, nil
}

// NewSnowflakeNode creates a new instance of SnowflakeNode with a unique host ID
func NewSnowflakeNode() (*snowflake.Node, error) {
	return snowflake.NewNode(int64(generateHostID()))
}

// generateHostID generates a unique host ID based on the container ID, hostname, and MAC address
// The generated host ID is a number between 0 and 1023 and is used to identify the host in a distributed system
// This should be used as the host ID for the snowflake ID generation algorithm
func generateHostID() int {
	containerID := getContainerID()
	hostname := getHostname()
	macAddress := getMACAddress()

	// Combine container ID, hostname, and MAC address
	combinedStr := fmt.Sprintf("%s-%s-%s", containerID, hostname, macAddress)

	// Hash the combined string using SHA-256
	hash := sha256.Sum256([]byte(combinedStr))

	// Convert the hashed value to an integer within the desired range (0-1023)
	return int(hash[0]) % 1024
}

// getContainerID retrieves the ID of the container in which the application is running
func getContainerID() string {
	// Logic to retrieve container ID
	// Example: Docker - return container ID, Kubernetes - return pod UID
	out, err := exec.Command("cat", "/proc/self/cgroup").Output()
	if err != nil {
		fmt.Println("Error retrieving container ID:", err)
		return "unknown"
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "docker") || strings.Contains(line, "kubepods") {
			parts := strings.Split(line, "/")
			return parts[len(parts)-1]
		}
	}

	return "unknown"
}

// getHostname retrieves the hostname of the machine
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("Error retrieving hostname:", err)
		return "unknown"
	}
	return hostname
}

// getMACAddress retrieves the MAC address of the network interface
func getMACAddress() string {
	// Logic to retrieve MAC address
	// Example: Retrieve MAC address of network interface
	return "00:00:00:00:00:00" // Placeholder implementation
}
