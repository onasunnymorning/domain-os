// Package config provides centralized configuration and environment variable management.
//
// The env var registry serves as the canonical source of truth for all environment
// variables used across the system. Each entry documents which services need it,
// whether it's required, and its default value.
//
// The accompanying test (env_registry_test.go) uses go/ast to scan the codebase for
// os.Getenv() calls and validates them against this registry, catching drift in CI.
package config

// Service identifies which component(s) use an env var.
type Service string

const (
	ServiceAPI      Service = "api"
	ServiceWorker   Service = "worker"
	ServiceFrontend Service = "frontend"
	ServiceEPP      Service = "epp"
	ServiceMCP      Service = "mcp"
	ServiceWhois    Service = "whois"
	ServiceCLI      Service = "cli"
)

// EnvVar documents a single environment variable.
type EnvVar struct {
	Name     string    // Variable name (e.g. "DATABASE_URL")
	Services []Service // Which services use this var
	Required bool      // If true, must be explicitly set (no sensible default)
	Default  string    // Default value when not required (empty = no default)

	// Secret marks the value as credential material. Deployment tooling uses it
	// to decide between a secret-store reference and a plaintext env var, so it
	// is declared explicitly per variable rather than inferred from the name.
	// A name heuristic fails silently and in the unsafe direction: "DB_PASS" does
	// not contain "PASSWORD", and "DATABASE_URL" embeds a password but matches
	// nothing at all. TestSecretsAreExplicit guards against the next such miss.
	Secret bool

	// RequiredWhen expresses a conditional requirement that Required cannot,
	// in the form "OTHER_VAR=value" (e.g. "STORAGE_AUTH_MODE=static"). It is
	// advisory: the contract emits it so consumers can validate their own config.
	RequiredWhen string

	Description string // Human-readable description
}

// Registry is the canonical list of all environment variables used by domain-os.
// When adding a new env var, add it here first. The drift test will fail if a
// variable is used in code but missing from this registry.
var Registry = []EnvVar{
	// ═══════════════════════════════════════════
	// DATABASE
	// ═══════════════════════════════════════════
	// DATABASE_URL is only understood by admin-api and unified-worker. whois and
	// mcp-server construct their DSN from the individual DB_* vars — see
	// cmd/whois/whois.go and cmd/mcp/main.go — so they must be given those.
	{Name: "DATABASE_URL", Services: []Service{ServiceAPI, ServiceWorker}, Required: true, Secret: true, Description: "PostgreSQL connection URL. Embeds the password, so it is credential material. Preferred over individual DB_* vars."},
	{Name: "DB_USER", Services: []Service{ServiceAPI, ServiceWorker, ServiceMCP, ServiceWhois, ServiceCLI}, Default: "postgres", Description: "PostgreSQL user (local/docker-compose fallback)"},
	{Name: "DB_PASS", Services: []Service{ServiceAPI, ServiceWorker, ServiceMCP, ServiceWhois, ServiceCLI}, Default: "postgres", Secret: true, Description: "PostgreSQL password (local/docker-compose fallback)"},
	{Name: "DB_HOST", Services: []Service{ServiceAPI, ServiceWorker, ServiceMCP, ServiceWhois, ServiceCLI}, Default: "localhost", Description: "PostgreSQL host (local/docker-compose fallback)"},
	{Name: "DB_PORT", Services: []Service{ServiceAPI, ServiceWorker, ServiceMCP, ServiceWhois, ServiceCLI}, Default: "5432", Description: "PostgreSQL port (local/docker-compose fallback)"},
	{Name: "DB_NAME", Services: []Service{ServiceAPI, ServiceWorker, ServiceMCP, ServiceWhois, ServiceCLI}, Default: "domain_os", Description: "PostgreSQL database name (local/docker-compose fallback)"},
	{Name: "DB_SSLMODE", Services: []Service{ServiceAPI, ServiceWorker, ServiceMCP, ServiceWhois, ServiceCLI}, Default: "disable", Description: "PostgreSQL SSL mode"},
	{Name: "AUTO_MIGRATE", Services: []Service{ServiceAPI}, Default: "false", Description: "Run GORM AutoMigrate on startup"},

	// ═══════════════════════════════════════════
	// AUTH0
	// ═══════════════════════════════════════════
	{Name: "AUTH0_ENABLED", Services: []Service{ServiceAPI, ServiceWorker}, Default: "false", Description: "Enable Auth0 JWT validation"},
	{Name: "AUTH0_DOMAIN", Services: []Service{ServiceAPI, ServiceWorker}, Description: "Auth0 tenant domain (e.g. dev-xxx.us.auth0.com)"},
	{Name: "AUTH0_AUDIENCE", Services: []Service{ServiceAPI, ServiceWorker}, Description: "Auth0 API identifier (e.g. https://api.alpaca-test)"},
	{Name: "AUTH0_WORKER_CLIENT_ID", Services: []Service{ServiceWorker}, Description: "Auth0 M2M client ID for worker (preferred over AUTH0_CLIENT_ID)"},
	{Name: "AUTH0_WORKER_CLIENT_SECRET", Services: []Service{ServiceWorker}, Required: true, Secret: true, Description: "Auth0 M2M client secret for worker"},
	{Name: "AUTH0_CLIENT_ID", Services: []Service{ServiceWorker}, Description: "Auth0 client ID fallback (if WORKER variant not set)"},
	{Name: "AUTH0_CLIENT_SECRET", Services: []Service{ServiceWorker}, Secret: true, Description: "Auth0 client secret fallback (if WORKER variant not set)"},
	{Name: "ADMIN_TOKEN", Services: []Service{ServiceAPI, ServiceWorker, ServiceCLI}, Secret: true, Description: "Legacy bearer token (fallback when Auth0 disabled)"},

	// ═══════════════════════════════════════════
	// API SERVER
	// ═══════════════════════════════════════════
	{Name: "API_URL", Services: []Service{ServiceWorker}, Description: "Full API URL (e.g. https://api.example.com). Preferred over API_HOST+API_PORT."},
	{Name: "API_HOST", Services: []Service{ServiceAPI, ServiceWorker}, Default: "localhost", Description: "API bind host / target host for worker calls"},
	{Name: "API_PORT", Services: []Service{ServiceAPI, ServiceWorker}, Default: "8080", Description: "API bind port / target port for worker calls"},
	{Name: "API_NAME", Services: []Service{ServiceAPI}, Default: "Domain OS Admin API", Description: "API name for Swagger docs"},
	{Name: "API_VERSION", Services: []Service{ServiceAPI}, Default: "0.0.0", Description: "API version for Swagger docs"},
	{Name: "GIN_MODE", Services: []Service{ServiceAPI}, Default: "debug", Description: "Gin framework mode (debug/release)"},
	{Name: "CORS_ALLOWED_ORIGINS", Services: []Service{ServiceAPI}, Default: "http://localhost:3000", Description: "Comma-separated allowed origins for CORS"},

	// ═══════════════════════════════════════════
	// TEMPORAL
	// ═══════════════════════════════════════════
	{Name: "TEMPORAL_HOST_PORT", Services: []Service{ServiceAPI, ServiceWorker}, Default: "localhost:7233", Description: "Temporal server address"},
	{Name: "TEMPORAL_NAMESPACE", Services: []Service{ServiceAPI, ServiceWorker}, Default: "default", Description: "Temporal namespace"},
	{Name: "TEMPORAL_API_KEY", Services: []Service{ServiceAPI, ServiceWorker}, Secret: true, Description: "Temporal Cloud API key (replaces mTLS certs)"},
	{Name: "TEMPORAL_CLIENT_KEY", Services: []Service{ServiceAPI, ServiceWorker}, Secret: true, Description: "Temporal mTLS client key PEM (legacy, use API key instead)"},
	{Name: "TEMPORAL_CLIENT_CERT", Services: []Service{ServiceAPI, ServiceWorker}, Secret: true, Description: "Temporal mTLS client cert PEM (legacy, use API key instead)"},
	{Name: "TEMPORAL_UI_URL", Services: []Service{ServiceAPI}, Default: "http://localhost:8233", Description: "Temporal UI URL for workflow links"},

	// ═══════════════════════════════════════════
	// STORAGE (S3 / MinIO / R2)
	// ═══════════════════════════════════════════
	// The MINIO_* names these replaced are still honoured as a deprecated
	// fallback for one release (see internal/infrastructure/storage/env.go).
	// They are deliberately absent from this registry so they stay out of the
	// generated deployment contract.
	// Required in every auth mode, including iam. The S3 client is minio-go, not
	// the AWS SDK: minio.New rejects an empty endpoint and never derives one from
	// the region. For AWS S3 use the regional hostname, e.g. s3.us-east-1.amazonaws.com.
	{Name: "STORAGE_ENDPOINT", Services: []Service{ServiceAPI, ServiceWorker}, Required: true, Description: "S3-compatible endpoint, hostname only, no scheme. AWS S3: s3.<region>.amazonaws.com. Required even when STORAGE_AUTH_MODE=iam — the client is minio-go and does not resolve an endpoint from the region. Falls back to deprecated MINIO_ENDPOINT"},
	{Name: "STORAGE_AUTH_MODE", Services: []Service{ServiceAPI, ServiceWorker}, Default: "static", Description: "How to obtain S3 credentials: \"static\" (access key + secret; MinIO, R2, S3) or \"iam\" (short-lived credentials from EKS IRSA, ECS task role, or EC2 IMDS; AWS S3 only)"},
	{Name: "STORAGE_ACCESS_KEY", Services: []Service{ServiceAPI, ServiceWorker}, Secret: true, RequiredWhen: "STORAGE_AUTH_MODE=static", Description: "S3 access key ID. Required when STORAGE_AUTH_MODE=static; must be unset for iam. Falls back to deprecated MINIO_ACCESS_KEY"},
	{Name: "STORAGE_SECRET_KEY", Services: []Service{ServiceAPI, ServiceWorker}, Secret: true, RequiredWhen: "STORAGE_AUTH_MODE=static", Description: "S3 secret access key. Required when STORAGE_AUTH_MODE=static; must be unset for iam. Falls back to deprecated MINIO_SECRET_KEY"},
	{Name: "STORAGE_USE_SSL", Services: []Service{ServiceAPI, ServiceWorker}, Default: "false", Description: "Use TLS for S3 connections. Falls back to deprecated MINIO_USE_SSL"},
	{Name: "STORAGE_REGION", Services: []Service{ServiceAPI, ServiceWorker}, Description: "S3 region for SigV4 signing. Set to the bucket's region for AWS S3, \"auto\" for R2. Empty lets the client resolve it via a bucket-location lookup"},
	{Name: "STORAGE_PUBLIC_ENDPOINT", Services: []Service{ServiceAPI}, Description: "Public S3 endpoint for presigned URLs (if different from STORAGE_ENDPOINT). Falls back to deprecated MINIO_PUBLIC_ENDPOINT"},
	{Name: "STORAGE_ESCROW_BUCKET", Services: []Service{ServiceAPI, ServiceWorker}, Default: "escrow", Description: "S3 bucket name for RDE/BRDA escrow deposits (contains PII — kept isolated per bucket-storage-strategy)"},
	{Name: "STORAGE_EVENT_LOGS_BUCKET", Services: []Service{ServiceAPI, ServiceWorker}, Default: "event-logs", Description: "S3 bucket name for gzip JSONL event archive files (warm-tier event search + audit trail)"},
	{Name: "STORAGE_REPORTS_BUCKET", Services: []Service{ServiceAPI, ServiceWorker}, Default: "reports", Description: "S3 bucket name for Spec 5 compliance sweep CSVs and other generated reports. API needs it too for the generic workflow artifact download endpoint"},
	{Name: "STORAGE_TEMP_BUCKET", Services: []Service{ServiceAPI, ServiceWorker}, Default: "temp-artifacts", Description: "S3 bucket name for workflow-scoped staging artifacts (snapshots, backups, cleanup manifests, verification reports). API needs it too for the generic workflow artifact download endpoint"},
	{Name: "STORAGE_TLS_SKIP_VERIFY", Services: []Service{ServiceAPI, ServiceWorker}, Default: "false", Description: "Disable TLS certificate verification for S3 connections. Local development against self-signed MinIO only — never enable against R2/S3"},
	{Name: "STORAGE_AUTO_CREATE_BUCKETS", Services: []Service{ServiceAPI, ServiceWorker}, Default: "false", Description: "Create missing storage buckets on startup. Local development only — production buckets are provisioned out-of-band"},
	{Name: "ESCROW_UPLOAD_DIR", Services: []Service{ServiceAPI}, Default: "/tmp/escrow-uploads", Description: "Local temp dir for escrow file uploads before S3 transfer"},

	// ═══════════════════════════════════════════
	// OBSERVABILITY
	// ═══════════════════════════════════════════
	{Name: "NEW_RELIC_ENABLED", Services: []Service{ServiceAPI}, Default: "false", Description: "Enable New Relic APM"},
	{Name: "NEW_RELIC_LICENSE_KEY", Services: []Service{ServiceAPI}, Secret: true, Description: "New Relic license key"},
	{Name: "PROMETHEUS_ENABLED", Services: []Service{ServiceAPI}, Default: "false", Description: "Enable Prometheus metrics endpoint"},
	{Name: "LOG_DOMAIN_EVENTS", Services: []Service{ServiceAPI}, Default: "true", Description: "Log domain lifecycle events"},
	{Name: "LOG_LEVEL", Services: []Service{ServiceEPP}, Default: "info", Description: "Log level (debug/info/warn/error)"},

	// ═══════════════════════════════════════════
	// EPP / REDIS
	// ═══════════════════════════════════════════
	{Name: "EPP_PORT", Services: []Service{ServiceEPP}, Default: "700", Description: "EPP server TCP listen port"},
	{Name: "REDIS_HOST", Services: []Service{ServiceEPP}, Default: "localhost", Description: "Redis host for EPP session store"},
	{Name: "REDIS_PORT", Services: []Service{ServiceEPP}, Default: "6379", Description: "Redis port"},
	{Name: "REDIS_PASSWORD", Services: []Service{ServiceEPP}, Secret: true, Description: "Redis password (empty = no auth)"},

	// ═══════════════════════════════════════════
	// MCP SERVER
	// ═══════════════════════════════════════════
	{Name: "MCP_TRANSPORT", Services: []Service{ServiceMCP}, Default: "stdio", Description: "MCP transport mode: 'stdio' for local IDE, 'http' for container deployment"},
	{Name: "MCP_PORT", Services: []Service{ServiceMCP}, Default: "3001", Description: "HTTP listen port when MCP_TRANSPORT=http"},

	// ═══════════════════════════════════════════
	// EXTERNAL APIs
	// ═══════════════════════════════════════════
	{Name: "OPENEXCHANGERATES_APP_ID", Services: []Service{ServiceWorker}, Secret: true, Description: "OpenExchangeRates API key for FX sync"},
	{Name: "ANTHROPIC_API_KEY", Services: []Service{ServiceAPI, ServiceCLI}, Secret: true, Description: "Anthropic API key for AI agent (Agent Alpaca)"},
	{Name: "ANTHROPIC_BASE_URL", Services: []Service{ServiceAPI, ServiceCLI}, Description: "Anthropic API base URL override"},
	{Name: "LLM_MODEL", Services: []Service{ServiceAPI, ServiceCLI}, Default: "claude-sonnet-4-6", Description: "LLM model name for the AI agent (e.g. claude-sonnet-4-6)"},
	{Name: "KNOWLEDGE_BASE_DIR", Services: []Service{ServiceAPI, ServiceCLI}, Description: "Root directory for knowledge base docs (docs/index.yaml). Falls back to working directory."},

	// ═══════════════════════════════════════════
	// FRONTEND (NEXT_PUBLIC_*)
	//
	// These are read at container start by next-runtime-env, not baked into the
	// JS bundle. Infra must set them on the running frontend container, not as
	// docker build args. See docs/adr/0001-runtime-env-configuration.md
	// ═══════════════════════════════════════════
	{Name: "NEXT_PUBLIC_API_URL", Services: []Service{ServiceFrontend}, Required: true, Description: "Backend API URL (read at container start)"},
	{Name: "NEXT_PUBLIC_API_TOKEN", Services: []Service{ServiceFrontend}, Description: "Static API token fallback when Auth0 is disabled. Publicly readable in the browser — never a real secret"},
	{Name: "NEXT_PUBLIC_AUTH0_ENABLED", Services: []Service{ServiceFrontend}, Default: "false", Description: "Enable Auth0 login flow"},
	{Name: "NEXT_PUBLIC_AUTH0_DOMAIN", Services: []Service{ServiceFrontend}, Description: "Auth0 tenant domain for SPA login"},
	{Name: "NEXT_PUBLIC_AUTH0_CLIENT_ID", Services: []Service{ServiceFrontend}, Description: "Auth0 SPA application client ID"},
	{Name: "NEXT_PUBLIC_AUTH0_AUDIENCE", Services: []Service{ServiceFrontend}, Description: "Auth0 API identifier for token audience"},
	{Name: "NEXT_PUBLIC_APP_VERSION", Services: []Service{ServiceFrontend}, Default: "0.0.0", Description: "App version displayed in UI. Injected at container start by next-runtime-env (not baked into the image); the deployment sets it to the release version"},
	{Name: "NEXT_PUBLIC_TEMPORAL_UI_URL", Services: []Service{ServiceFrontend}, Description: "Temporal Cloud UI URL for workflow links"},
	{Name: "NEXT_PUBLIC_TEMPORAL_NAMESPACE", Services: []Service{ServiceFrontend}, Default: "default", Description: "Temporal namespace used to build schedule deep-links"},
	{Name: "NEXT_PUBLIC_STORAGE_UI_URL", Services: []Service{ServiceFrontend}, Description: "S3 storage browser URL"},
	{Name: "NEXT_PUBLIC_POSTHOG_PROJECT_TOKEN", Services: []Service{ServiceFrontend}, Description: "PostHog project token. Publicly readable in the browser — never a real secret"},
	{Name: "NEXT_PUBLIC_POSTHOG_HOST", Services: []Service{ServiceFrontend}, Default: "https://us.i.posthog.com", Description: "PostHog ingestion host"},

	// ═══════════════════════════════════════════
	// RUNTIME DETECTION (read-only, set by platform)
	// ═══════════════════════════════════════════
	{Name: "LAMBDA_TASK_ROOT", Services: []Service{ServiceAPI}, Description: "Set by AWS Lambda runtime — used to detect Lambda environment"},
	{Name: "KUBERNETES_SERVICE_HOST", Services: []Service{ServiceAPI}, Description: "Set by K8s — used to detect Kubernetes environment"},
}

// RegistryMap returns the registry as a map keyed by variable name for O(1) lookups.
func RegistryMap() map[string]EnvVar {
	m := make(map[string]EnvVar, len(Registry))
	for _, e := range Registry {
		m[e.Name] = e
	}
	return m
}
