package services

import "github.com/onasunnymorning/domain-os/pkg/domain/entities"

// testPlatformScope is the scope tests use for fixture setup and teardown,
// where no operator is involved.
var testPlatformScope = entities.PlatformRegistryScope(entities.NewPlatformScope())
