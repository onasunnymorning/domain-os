package snowflakeidgenerator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The process has one node, so the tests that touch it run in one order:
// SetNodeID first, then every generator is on that node.
func TestSetNodeID(t *testing.T) {
	require.Error(t, SetNodeID(-1))
	require.Error(t, SetNodeID(MaxNodeID+1))

	require.NoError(t, SetNodeID(7))
	require.NoError(t, SetNodeID(7), "naming the node the process already has is not an error")
	err := SetNodeID(8)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already on snowflake node 7")

	g, err := NewIDGenerator()
	require.NoError(t, err)
	assert.Equal(t, int64(7), g.ListNode())
	assert.NotEqual(t, g.GenerateID(), g.GenerateID())
}

func TestHostIDFromParts(t *testing.T) {
	seen := map[int]bool{}
	for _, parts := range [][3]string{
		{"unknown", "a", "00:00:00:00:00:00"},
		{"unknown", "b", "00:00:00:00:00:00"},
		{"c1", "a", "00:00:00:00:00:00"},
	} {
		id := hostIDFromParts(parts[0], parts[1], parts[2])
		assert.GreaterOrEqual(t, id, 0)
		assert.LessOrEqual(t, id, MaxNodeID)
		seen[id] = true
	}
	assert.Len(t, seen, 3, "the sample parts hash to distinct nodes")
	assert.Equal(t, hostIDFromParts("x", "y", "z"), hostIDFromParts("x", "y", "z"), "deterministic")
	// Ten bits, not one byte: some inputs land above 255.
	above := false
	for _, h := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		if hostIDFromParts("unknown", h, "") > 255 {
			above = true
		}
	}
	assert.True(t, above)
}
