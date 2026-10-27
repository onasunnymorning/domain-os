package snowflakeidgenerator

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

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

// The process has one snowflake node, built on first use and shared by
// every IDGenerator. Two nodes with the same host ID in one process could
// mint the same ID in the same millisecond, and snowflake.NewNode writes the
// library's package-level masks, which every Generate reads: building nodes
// concurrently, as the worker's import activities did, raced on them.
var (
	nodeOnce sync.Once
	node     *snowflake.Node
	nodeID   int64
	nodeErr  error
)

// MaxNodeID is the largest node ID: the snowflake node field is ten bits.
const MaxNodeID = 1023

// SetNodeID fixes the process's snowflake node to id, 0 to MaxNodeID, before
// the first generator is built. The composition root calls it with
// SNOWFLAKE_NODE_ID when that is set: a node derived from the host (see
// NewIDGenerator) is a gamble for a fleet, and two processes on one node can
// mint the same ROID in the same millisecond. A second call, or a call after
// a generator was built on a derived node, is an error unless it names the
// node the process already has.
func SetNodeID(id int64) error {
	if id < 0 || id > MaxNodeID {
		return fmt.Errorf("snowflake node %d is not in 0 to %d", id, MaxNodeID)
	}
	nodeOnce.Do(func() {
		node, nodeErr = snowflake.NewNode(id)
		nodeID = id
	})
	if nodeErr != nil {
		return nodeErr
	}
	if nodeID != id {
		return fmt.Errorf("the process is already on snowflake node %d, not %d", nodeID, id)
	}
	return nil
}

// NewIDGenerator returns an IDGenerator on the process's snowflake node: the
// one SetNodeID fixed, or else one derived from the host on first use.
func NewIDGenerator() (*IDGenerator, error) {
	nodeOnce.Do(func() {
		node, nodeErr = NewSnowflakeNode()
		if nodeErr == nil {
			nodeID = node.Generate().Node()
		}
	})
	if nodeErr != nil {
		return nil, nodeErr
	}
	return &IDGenerator{node: node, nodeID: nodeID}, nil
}

// NewSnowflakeNode creates a snowflake node on an ID derived from the host.
// NewIDGenerator calls it once per process; a caller that builds nodes of
// its own must not do so concurrently with any Generate (see above).
func NewSnowflakeNode() (*snowflake.Node, error) {
	return snowflake.NewNode(int64(generateHostID()))
}

// generateHostID derives a node ID, 0 to MaxNodeID, from the container ID,
// hostname and MAC address. It is a hash, so two hosts can share one: with
// ten replicas the odds are about one in twenty. A fleet sets
// SNOWFLAKE_NODE_ID instead.
func generateHostID() int {
	return hostIDFromParts(getContainerID(), getHostname(), getMACAddress())
}

// hostIDFromParts is generateHostID on the given parts: ten bits of their
// SHA-256. Until 2026-10-27 it was one byte of the hash, so only
// 256 of the 1024 node IDs could ever be used.
func hostIDFromParts(containerID, hostname, macAddress string) int {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%s-%s", containerID, hostname, macAddress)))
	return int(binary.BigEndian.Uint16(hash[:2]) % (MaxNodeID + 1))
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
