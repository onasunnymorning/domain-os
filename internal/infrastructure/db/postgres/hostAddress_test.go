package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostAddress_Tablename(t *testing.T) {
	hostAddress := HostAddress{}
	require.Equal(t, "host_addresses", hostAddress.TableName())
}
