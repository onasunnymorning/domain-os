// Command epp runs the registrar-facing EPP server. It is the composition
// root only: configuration is read here (INV-17) and everything else lives in
// internal/interface/epp. See docs/epp/ARCHITECTURE.md.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/buildinfo"
	"github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/epp/lockout"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/epp/sessionstore"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/passwordhash"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/ratelimit"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/internal/interface/epp"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func main() {
	cfg, err := config.LoadEPP()
	if err != nil {
		slog.Error("EPP server configuration is invalid", "error", err)
		os.Exit(1)
	}
	// The server mints ROIDs on <domain:create> and shows them on
	// <domain:info>, so it does not start without the registry's repository
	// ID (ADR 0017).
	repositoryID, err := config.LoadRepositoryID()
	if err != nil {
		slog.Error("EPP server configuration is invalid", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	logger.Info("EPP server starting", "version", buildinfo.Version, "git_sha", buildinfo.GitSHA, "node", cfg.Sessions.NodeID, "repository_id", repositoryID.String())

	if err := run(cfg, repositoryID, logger); err != nil {
		logger.Error("EPP server stopped", "error", err)
		os.Exit(1)
	}
}

// useFlushInterval is how often last-used times are written: at most one
// write per credential per interval, however many logins (ADR 0014).
const useFlushInterval = time.Minute

// auditPruneInterval is how often audit records past retention are deleted.
// Every node prunes; the deletes are idempotent.
const auditPruneInterval = time.Hour

func run(cfg config.EPP, repositoryID entities.RepositoryID, logger *slog.Logger) error {
	if cfg.TLS.SelfSigned {
		logger.Warn("EPP server is presenting a self-signed certificate (EPP_TLS_SELF_SIGNED=true); local development only")
	}
	if !cfg.Auth.RequireAllowlist {
		logger.Warn("EPP source allowlists are not enforced (EPP_REQUIRE_ALLOWLIST=false); local development only")
	}

	// Logins, the credential refresh, checks and the audit log share a
	// bounded pool, so a node cannot crowd out the API and workers.
	db, err := postgres.NewConnection(postgres.Config{
		User: cfg.DB.User, Pass: cfg.DB.Pass, Host: cfg.DB.Host, Port: cfg.DB.Port,
		DBName: cfg.DB.Name, SSLmode: cfg.DB.SSLMode, MaxOpenConns: cfg.DB.MaxOpenConns,
	})
	if err != nil {
		return fmt.Errorf("connect to the database at %s:%s: %w", cfg.DB.Host, cfg.DB.Port, err)
	}
	schemaLog := func(format string, args ...any) { logger.Info(fmt.Sprintf(format, args...)) }
	if err := postgres.EnforceSchemaGuard(db, buildinfo.Version, postgres.SchemaGuardModeFromEnv(), schemaLog); err != nil {
		return err
	}
	if err := postgres.CheckRepositoryID(context.Background(), db, repositoryID); err != nil {
		return err
	}
	logger.Info("Connected to the database", "host", cfg.DB.Host, "name", cfg.DB.Name)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       0,
		// Every caller bounds its calls (the session registry and lockouts
		// to 500ms, command rates to 200ms) and falls back to the node when
		// Redis does not answer in time; without this the client ignores
		// those deadlines on an open connection and waits out its own
		// 3-second read timeout.
		ContextTimeoutEnabled: true,
	})
	// Redis holds the session registry, lockouts and command rates, and
	// the server fails open without it (ADR 0013), so an unreachable Redis
	// is a warning, not a reason to stay down. epp_policy_store_degraded
	// shows it.
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn("Redis unreachable at start; session caps, lockouts and command rates apply per node until it answers", "addr", cfg.Redis.Addr(), "error", err)
	} else {
		logger.Info("Connected to Redis", "addr", cfg.Redis.Addr())
	}

	if err := fixSnowflakeNode(); err != nil {
		return err
	}
	ids, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		return fmt.Errorf("id generator: %w", err)
	}
	logger.Info("Snowflake node", "node", ids.ListNode(), "from", snowflakeNodeSource())

	credentials := postgres.NewEPPCredentialRepository(db)
	uses := services.NewEPPCredentialUseRecorder(credentials)
	// srv reports the lockout store's health once it exists; until then
	// there have been no logins to report on.
	var srv *epp.Server
	lockouts := lockout.NewFailOpen(
		lockout.NewRedis(redisClient, lockout.DefaultPolicy),
		lockout.NewMemory(lockout.DefaultPolicy),
		func(degraded bool) {
			if srv != nil {
				srv.ReportLockoutDegraded(degraded)
			}
		})
	auth := services.NewEPPAuthenticationService(credentials, passwordhash.New(passwordhash.DefaultParams), lockouts, uses, cfg.Auth.MaxConcurrentVerify)

	// Command rates and create quotas (ADR 0019), counted in Redis and on
	// the node while Redis cannot answer.
	rates := ratelimit.NewFailOpen(ratelimit.NewRedis(redisClient), ratelimit.NewMemory(), func(degraded bool) {
		if srv != nil {
			srv.ReportRatesDegraded(degraded)
		}
	})

	// The failed-create budget (Phase 6a): counts, strikes and suspensions
	// in Redis, on the node while it cannot answer; daily counts per
	// registrar in the database, written every useFlushInterval.
	failedCreates := ratelimit.NewFailOpenFailedCreates(ratelimit.NewRedis(redisClient), ratelimit.NewMemoryFailedCreates(), func(degraded bool) {
		if srv != nil {
			srv.ReportFailedCreatesDegraded(degraded)
		}
	})
	outcomeRepo := postgres.NewEPPCreateOutcomeRepository(db)
	outcomes := services.NewEPPCreateOutcomeRecorder(outcomeRepo)

	sessions := services.NewEPPSessionService(sessionstore.NewRedis(redisClient), entities.EPPSessionCaps{
		PerSource: cfg.Sessions.PerSource,
	}, cfg.Sessions.Lease, nil)

	domainRepo := postgres.NewDomainRepository(db, repositoryID)
	hostRepo := postgres.NewGormHostRepository(db, repositoryID)
	nndnRepo := postgres.NewGormNNDNRepository(db)
	tldRepo := postgres.NewGormTLDRepo(db)
	phaseRepo := postgres.NewGormPhaseRepository(db)
	registrarRepo := postgres.NewGormRegistrarRepository(db)
	roids := services.NewRoidService(ids, repositoryID)
	events := postgres.NewPostgresEventPublisher(db, zap.NewNop(), false) // events are not logged here; the EPP audit log records the command
	// Registration goes through DomainService, as from the admin API: it
	// prices the domain, assigns its ROID, stores it and writes its event.
	registry := services.NewDomainService(domainRepo, hostRepo, *roids, nndnRepo, tldRepo, phaseRepo,
		postgres.NewGORMPremiumLabelRepository(db), postgres.NewFXRepository(db), registrarRepo, events, postgres.NewTransactor(db))
	// Transfers and the poll queue share a repository: a transfer's writes
	// queue its messages in the same transaction.
	transfers := postgres.NewDomainTransferRepository(db, repositoryID)
	registry.SetTransferRepo(transfers)
	domains := services.NewEPPDomainService(services.EPPDomainServiceDeps{
		Domains: domainRepo, NNDNs: nndnRepo, TLDs: tldRepo, Phases: phaseRepo,
		Contacts: postgres.NewContactRepository(db, repositoryID), Hosts: hostRepo, Registrars: registrarRepo,
		Transfers: transfers, Register: registry,
	})
	hosts := services.NewEPPHostService(services.EPPHostServiceDeps{
		Hosts: hostRepo, Domains: domainRepo, TLDs: tldRepo, Roids: roids, Events: events,
	})
	contacts := services.NewEPPContactService(services.EPPContactServiceDeps{
		Contacts: postgres.NewContactRepository(db, repositoryID), Roids: roids, Events: events,
	})
	audit := services.NewEPPAuditService(postgres.NewEPPTransactionRepository(db), cfg.Audit.Buffer)

	srv, err = epp.NewServer(epp.Options{
		Config:        cfg,
		Authenticator: auth,
		Sessions:      sessions,
		Domains:       domains,
		Hosts:         hosts,
		Contacts:      contacts,
		Polls:         services.NewEPPPollService(transfers),
		// The registrars' EPP policies (ADR 0019), reloaded with the
		// credentials.
		Policies:       services.NewEPPPolicyService(postgres.NewEPPPolicyRepository(db), registrarRepo, outcomeRepo),
		Rates:          rates,
		FailedCreates:  failedCreates,
		CreateOutcomes: outcomes,
		Events:         events,
		Audit:          audit,
		Logger:         logger,
		NewID:          func() string { return strconv.FormatInt(ids.GenerateID(), 10) },
	})
	if err != nil {
		return err
	}

	flushCtx, stopFlush := context.WithCancel(context.Background())
	defer stopFlush()
	flushed := make(chan struct{})
	go func() {
		defer close(flushed)
		flushUses(flushCtx, uses, outcomes, logger)
	}()

	// The audit writer outlives the drain: commands finishing during it
	// still queue records, which it writes before it stops.
	auditCtx, stopAudit := context.WithCancel(context.Background())
	defer stopAudit()
	audited := make(chan struct{})
	go func() {
		defer close(audited)
		audit.Run(auditCtx, 10*time.Second, srv.ReportAuditError)
	}()
	go pruneAudit(flushCtx, audit, cfg.Audit.Retention, logger)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("listen on port %d: %w", cfg.Port, err)
	}

	ops := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.OpsPort),
		Handler:           srv.OpsHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	opsErr := make(chan error, 1)
	go func() {
		logger.Info("EPP ops endpoint listening", "addr", ops.Addr)
		if err := ops.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			opsErr <- fmt.Errorf("ops endpoint: %w", err)
		}
	}()

	served := make(chan error, 1)
	go func() { served <- srv.Serve(listener) }()

	stop, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	var runErr error
	select {
	case <-stop.Done():
		logger.Info("EPP server received shutdown signal", "drain_timeout", cfg.Timeouts.Drain.String())
	case runErr = <-served:
		served = nil
	case runErr = <-opsErr:
	}

	drainCtx, cancelDrain := context.WithTimeout(context.Background(), cfg.Timeouts.Drain)
	defer cancelDrain()
	if err := srv.Shutdown(drainCtx); err != nil {
		logger.Warn("EPP drain did not finish cleanly", "error", err)
	}
	if served != nil {
		if err := <-served; err != nil && runErr == nil {
			runErr = err
		}
	}

	stopFlush()
	<-flushed
	stopAudit()
	<-audited

	opsCtx, cancelOps := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelOps()
	_ = ops.Shutdown(opsCtx)
	return runErr
}

// flushUses writes last-used times and the daily create counts every
// useFlushInterval, and once more when ctx is done, so a drained node does
// not lose its last minute.
func flushUses(ctx context.Context, uses *services.EPPCredentialUseRecorder, outcomes *services.EPPCreateOutcomeRecorder, logger *slog.Logger) {
	ticker := time.NewTicker(useFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			final, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := uses.Flush(final); err != nil {
				logger.Warn("EPP credential last-used times not written at shutdown", "error", err)
			}
			if err := outcomes.Flush(final); err != nil {
				logger.Warn("EPP daily create counts not written at shutdown", "error", err)
			}
			cancel()
			return
		}
		if err := uses.Flush(ctx); err != nil {
			logger.Warn("EPP credential last-used times not written; retrying next interval", "error", err)
		}
		if err := outcomes.Flush(ctx); err != nil {
			logger.Warn("EPP daily create counts not written; retrying next interval", "error", err)
		}
	}
}

// pruneAudit deletes audit records past retention every
// auditPruneInterval, starting at once.
func pruneAudit(ctx context.Context, audit *services.EPPAuditService, retention time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(auditPruneInterval)
	defer ticker.Stop()
	for {
		n, err := audit.Prune(ctx, retention)
		switch {
		case err != nil && ctx.Err() == nil:
			logger.Warn("EPP audit records past retention not deleted; retrying next interval", "deleted", n, "error", err)
		case n > 0:
			logger.Info("EPP audit records past retention deleted", "deleted", n, "retention", retention.String())
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

// fixSnowflakeNode puts the process on SNOWFLAKE_NODE_ID when it is set.
func fixSnowflakeNode() error {
	id, set, err := config.LoadSnowflakeNodeID()
	if err != nil {
		return err
	}
	if !set {
		return nil
	}
	return snowflakeidgenerator.SetNodeID(id)
}

// snowflakeNodeSource says where the process's node came from, for the log.
func snowflakeNodeSource() string {
	if _, set, _ := config.LoadSnowflakeNodeID(); set {
		return "SNOWFLAKE_NODE_ID"
	}
	return "derived from the host; set SNOWFLAKE_NODE_ID per replica"
}
