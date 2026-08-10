package config

import (
	"fmt"
	"strings"
)

// Generating .env.example.
//
// The registry below is the only place env vars are declared, so .env.example
// is generated from it rather than hand-maintained. A hand-written example file
// drifts the moment someone adds a variable in code, and the drift is silent —
// the newcomer just gets a nil panic three minutes in. TestEnvExampleDrift
// fails the build instead.
//
// Regenerate with:  make generate-env-example

// envExampleLocalValues overrides a variable's registry Default for local
// development. The registry Default is the *production* default; these are the
// values that make `make dev` work against docker-compose with no edits.
//
// Anything not listed here uses its registry Default. Anything with no registry
// Default is emitted empty.
var envExampleLocalValues = map[string]string{
	// The API owns schema creation locally — there is no migration tool, so
	// nothing else will create the tables on a first-ever run.
	"AUTO_MIGRATE": "true",

	// Static bearer token for the local API. Not a secret: it guards a database
	// full of generated .test domains on your own laptop.
	"ADMIN_TOKEN": "devtoken",

	// Verbose request logging is what you want locally.
	"GIN_MODE":  "debug",
	"LOG_LEVEL": "debug",

	// Service addresses as seen from inside the compose network.
	"TEMPORAL_HOST_PORT": "temporal:7233",
	"STORAGE_ENDPOINT":   "minio:9000",
	"REDIS_HOST":         "redis",

	// Bind address, not a hostname. The registry default "localhost" is correct
	// for a process on your host but wrong inside a container — it binds the
	// loopback interface, so the published port reaches nothing.
	"API_HOST": "0.0.0.0",

	// Presigned S3 URLs are handed to a browser, so they must name the endpoint
	// as the *host* sees it, not as the compose network does.
	"STORAGE_PUBLIC_ENDPOINT": "http://localhost:9000",

	// MinIO's built-in development credentials. These are the documented
	// defaults of the local minio container, not a leaked key.
	"STORAGE_ACCESS_KEY": "minioadmin",
	"STORAGE_SECRET_KEY": "minioadmin",

	// The compose stack provisions buckets via minio-setup, so the worker does
	// not need to create them itself.
	"STORAGE_AUTO_CREATE_BUCKETS": "false",

	// Browser-facing URLs. The frontend dev server runs on 3002 to stay clear
	// of Grafana on 3000.
	"CORS_ALLOWED_ORIGINS":        "http://localhost:3000,http://localhost:3002",
	"NEXT_PUBLIC_API_URL":         "http://localhost:8080",
	"NEXT_PUBLIC_API_TOKEN":       "devtoken",
	"NEXT_PUBLIC_TEMPORAL_UI_URL": "http://localhost:8081",
	"NEXT_PUBLIC_STORAGE_UI_URL":  "http://localhost:9001",
	"TEMPORAL_UI_URL":             "http://localhost:8081",
}

// envExampleSkip lists variables the platform injects at runtime. Putting them
// in .env.example would invite someone to set them by hand, which breaks the
// runtime detection that reads them.
var envExampleSkip = map[string]bool{
	"LAMBDA_TASK_ROOT":        true,
	"KUBERNETES_SERVICE_HOST": true,
}

const envExampleHeader = `# ============================================================================
# domain-os — local development environment
# ============================================================================
#
# GENERATED FILE — do not edit by hand.
#   Source:      internal/config/env_registry.go
#   Regenerate:  make generate-env-example
#   Guarded by:  TestEnvExampleDrift
#
# Quick start:
#
#   cp .env.example .env
#
# That is the whole configuration step. Every value below is a working local
# default; you do not need to change any of them to run 'make dev'.
#
# LOCAL DEVELOPMENT REQUIRES NO CLOUD CREDENTIALS. There is no AWS key, no
# Doppler login, no Grafana Cloud token, and no Auth0 tenant in the local boot
# or test path. Every [SECRET] below is either a local container's documented
# default or is left deliberately empty because local does not need it.
#
# Annotations:
#   [REQUIRED]  the service will not start without it
#   [OPTIONAL]  has a working default
#   [SECRET]    credential material in production — never commit a real value
#   [CLOUD]     only needed when pointing at a hosted service; ignore locally
# ============================================================================

# Docker image tag for the app services. Compose falls back to "latest" when
# unset; 'make dev' builds these locally so the tag is only a local cache label.
BRANCH="latest"

`

// GenerateEnvExample renders the .env.example file from the registry.
func GenerateEnvExample() (string, error) {
	var b strings.Builder
	b.WriteString(envExampleHeader)

	for _, v := range Registry {
		if envExampleSkip[v.Name] {
			continue
		}

		value, explicit := envExampleLocalValues[v.Name]
		if !explicit {
			value = v.Default
		}

		for _, line := range wrapComment(v.Description, 74) {
			b.WriteString("# " + line + "\n")
		}

		if v.RequiredWhen != "" {
			b.WriteString(fmt.Sprintf("# Required when %s\n", v.RequiredWhen))
		}
		b.WriteString(fmt.Sprintf("# Used by: %s\n", servicesLabel(v.Services)))
		b.WriteString(fmt.Sprintf("# %s\n", annotations(v, value)))
		b.WriteString(fmt.Sprintf("%s=%q\n\n", v.Name, value))
	}

	return b.String(), nil
}

// annotations renders the bracket tags for one variable.
func annotations(v EnvVar, value string) string {
	tags := make([]string, 0, 3)
	if v.Required {
		tags = append(tags, "[REQUIRED]")
	} else {
		tags = append(tags, "[OPTIONAL]")
	}
	if v.Secret {
		tags = append(tags, "[SECRET]")
	}
	// A secret left empty locally is a deliberate signal: local does not talk to
	// whatever it authenticates against.
	if v.Secret && value == "" {
		tags = append(tags, "[CLOUD] — not needed for local development")
	}
	return strings.Join(tags, " ")
}

func servicesLabel(services []Service) string {
	if len(services) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(services))
	for _, s := range services {
		names = append(names, string(s))
	}
	return strings.Join(names, ", ")
}

// wrapComment breaks a description into lines of at most width characters so
// the generated file stays readable in a terminal.
func wrapComment(text string, width int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{"(no description)"}
	}

	words := strings.Fields(text)
	lines := make([]string, 0, 4)
	current := ""
	for _, w := range words {
		switch {
		case current == "":
			current = w
		case len(current)+1+len(w) <= width:
			current += " " + w
		default:
			lines = append(lines, current)
			current = w
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
