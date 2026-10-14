package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// LoadRepositoryID reads ROID_REPOSITORY_ID: the registry's EPP repository
// identifier, registered with IANA, which every ROID the registry mints ends
// in (ADR 0017). Every process that mints ROIDs — the admin API and its seed,
// the EPP server, the worker and the JISC importer — calls it at start and
// refuses to start on an error. There is no default: a registry's objects
// would carry it in escrow deposits and over EPP, and a wrong one cannot be
// taken back, because an object keeps its ROID.
func LoadRepositoryID() (entities.RepositoryID, error) {
	v := strings.TrimSpace(os.Getenv("ROID_REPOSITORY_ID"))
	if v == "" {
		return "", fmt.Errorf("ROID_REPOSITORY_ID is not set: set the registry's IANA-registered EPP repository ID, " +
			"which every new ROID ends in (any 1 to 8 letters or digits for local development; see docs/epp/POLICY_REFERENCE.md)")
	}
	id, err := entities.NewRepositoryID(v)
	if err != nil {
		return "", fmt.Errorf("ROID_REPOSITORY_ID: %w", err)
	}
	return id, nil
}
