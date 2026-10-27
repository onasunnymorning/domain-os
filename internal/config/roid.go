package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// LoadRepositoryID reads ROID_REPOSITORY_ID: the registry's EPP repository
// identifier, registered with IANA, which every ROID ends in (ADR 0017). Only
// a ROID's number is stored and the ID is appended when it is read, so every
// process that mints or shows ROIDs — the admin API and its seed, the EPP
// server, the worker, whois, MCP, askg and the JISC importer — calls it at
// start and refuses to start on an error. There is no default: one would
// reach production unnoticed. postgres.CheckRepositoryID then holds every
// process to the ID the installation recorded on its first start.
func LoadRepositoryID() (entities.RepositoryID, error) {
	v := strings.TrimSpace(os.Getenv("ROID_REPOSITORY_ID"))
	if v == "" {
		return "", fmt.Errorf("ROID_REPOSITORY_ID is not set: set the registry's IANA-registered EPP repository ID, " +
			"which every ROID ends in (any 1 to 8 letters or digits for local development; see docs/epp/POLICY_REFERENCE.md)")
	}
	id, err := entities.NewRepositoryID(v)
	if err != nil {
		return "", fmt.Errorf("ROID_REPOSITORY_ID: %w", err)
	}
	return id, nil
}

// MaxSnowflakeNodeID is the largest snowflake node ID: the node field is ten
// bits wide.
const MaxSnowflakeNodeID = 1023

// LoadSnowflakeNodeID reads SNOWFLAKE_NODE_ID: the process's snowflake node,
// 0 to MaxSnowflakeNodeID, which every ROID it mints carries. Two processes
// on the same node can mint the same ROID in the same millisecond, so each
// replica that mints — the admin API and its seed, the EPP server, the
// worker and the JISC importer — is given a node of its own (a StatefulSet
// ordinal, say). Unset is not an error: the generator then derives a node
// from the host, which is fine for one process and a gamble for a fleet.
func LoadSnowflakeNodeID() (id int64, set bool, err error) {
	v := strings.TrimSpace(os.Getenv("SNOWFLAKE_NODE_ID"))
	if v == "" {
		return 0, false, nil
	}
	id, err = strconv.ParseInt(v, 10, 64)
	if err != nil || id < 0 || id > MaxSnowflakeNodeID {
		return 0, false, fmt.Errorf("SNOWFLAKE_NODE_ID=%q is not an integer from 0 to %d", v, MaxSnowflakeNodeID)
	}
	return id, true, nil
}
