package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/ianaregistrars"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/icannspec5"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
)

// TestAPI is the shared test harness for API integration tests.
// It holds the Gin router with all controllers registered, a live DB connection,
// and all services needed to set up prerequisite data.
type TestAPI struct {
	Router *gin.Engine
	Server *httptest.Server
	DB     *gorm.DB

	// Services exposed for test data setup (creating prerequisite entities directly)
	RoidService              *services.RoidService
	RegistryOperatorService  *services.RegistryOperatorService
	TLDService               *services.TLDService
	DomainService            *services.DomainService
	HostService              *services.HostService
	RegistrarService         *services.RegistrarService
	ContactService           *services.ContactService
	PhaseService             *services.PhaseService
	IANARegistrarService     *services.IANARegistrarService
	AccreditationService     *services.AccreditationService
	FeeService               *services.FeeService
	PriceService             *services.PriceService
	FXService                *services.FXService
	NNDNService              *services.NNDNService
	PremiumListService       *services.PremiumListService
	PremiumLabelService      *services.PremiumLabelService
	WhoisService             *services.WhoisService
}

// MockAuthMiddleware returns a gin.HandlerFunc that bypasses authentication.
func MockAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// NewTestAPI creates a fully wired TestAPI with all controllers registered.
// It connects to a real Postgres database (configured via env vars) and
// auto-migrates the schema.
func NewTestAPI() (*TestAPI, error) {
	gin.SetMode(gin.TestMode)

	db, err := newTestDB()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	// ID generator for ROID generation
	idGenerator, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		return nil, fmt.Errorf("failed to create ID generator: %w", err)
	}
	roidService := services.NewRoidService(idGenerator)

	// --- Repositories ---
	registryOperatorRepo := postgres.NewGORMRegistryOperatorRepository(db)
	tldRepo := postgres.NewGormTLDRepo(db)
	dnsRecordRepo := postgres.NewGormDNSRecordRepository(db)
	domainRepo := postgres.NewDomainRepository(db)
	hostRepo := postgres.NewGormHostRepository(db)
	hostAddressRepo := postgres.NewGormHostAddressRepository(db)
	registrarRepo := postgres.NewGormRegistrarRepository(db)
	contactRepo := postgres.NewContactRepository(db)
	phaseRepo := postgres.NewGormPhaseRepository(db)
	ianaRegistrarRepo := postgres.NewIANARegistrarRepository(db)
	accreditationRepo := postgres.NewAccreditationRepository(db)
	feeRepo := postgres.NewFeeRepository(db)
	priceRepo := postgres.NewGormPriceRepository(db)
	fxRepo := postgres.NewFXRepository(db)
	nndnRepo := postgres.NewGormNNDNRepository(db)
	premiumListRepo := postgres.NewGORMPremiumListRepository(db)
	premiumLabelRepo := postgres.NewGORMPremiumLabelRepository(db)
	spec5Repo := postgres.NewSpec5Repository(db)
	icannRepo := icannspec5.NewICANNRepo()
	ianaRepo := ianaregistrars.NewIANARRepository()

	// --- Services ---
	registryOperatorService := services.NewRegistryOperatorService(registryOperatorRepo)
	tldService := services.NewTLDService(tldRepo, dnsRecordRepo)
	domainService := services.NewDomainService(domainRepo, hostRepo, *roidService, nndnRepo, tldRepo, phaseRepo, premiumLabelRepo, fxRepo, registrarRepo)
	hostService := services.NewHostService(hostRepo, hostAddressRepo, roidService)
	registrarService := services.NewRegistrarService(registrarRepo)
	contactService := services.NewContactService(contactRepo, *roidService)
	phaseService := services.NewPhaseService(phaseRepo, tldRepo)
	ianaRegistrarService := services.NewIANARegistrarService(ianaRegistrarRepo)
	accreditationService := services.NewAccreditationService(accreditationRepo, registrarRepo, tldRepo)
	feeService := services.NewFeeService(phaseRepo, feeRepo)
	priceService := services.NewPriceService(phaseRepo, priceRepo)
	fxService := services.NewFXService(fxRepo)
	nndnService := services.NewNNDNService(nndnRepo)
	premiumListService := services.NewPremiumListService(premiumListRepo)
	premiumLabelService := services.NewPremiumLabelService(premiumLabelRepo)
	whoisService := services.NewWhoisService(domainRepo, registrarRepo)
	spec5Service := services.NewSpec5Service(spec5Repo)
	syncService := services.NewSyncService(ianaRegistrarRepo, spec5Repo, icannRepo, ianaRepo, fxRepo)

	// --- Router + Controllers ---
	router := gin.New()
	// ContextWithFallback makes gin.Context.Done()/Err()/Value() delegate to
	// Request.Context(), which prevents a data race between Gin's sync.Pool
	// recycling contexts and GORM's background goroutines (sql.Rows.awaitDone)
	// that still reference the previous request's context.
	router.ContextWithFallback = true
	auth := MockAuthMiddleware()

	rest.NewPingController(router)
	rest.NewRegistryOperatorController(router, registryOperatorService, auth)
	rest.NewTLDController(router, tldService, domainService, auth)
	rest.NewDomainController(router, domainService, auth)
	rest.NewHostController(router, hostService, auth)
	rest.NewRegistrarController(router, registrarService, ianaRegistrarService, auth)
	rest.NewContactController(router, contactService, auth)
	rest.NewPhaseController(router, phaseService, auth)
	rest.NewFeeController(router, feeService, auth)
	rest.NewPriceController(router, priceService, auth)
	rest.NewAccreditationController(router, accreditationService, auth)
	rest.NewPremiumController(router, premiumListService, premiumLabelService, auth)
	rest.NewFXController(router, fxService, auth)
	rest.NewNNDNController(router, nndnService, auth)
	rest.NewWhoisController(router, whoisService, auth)
	rest.NewSyncController(router, syncService, auth)
	rest.NewSpec5Controller(router, spec5Service, auth)
	rest.NewIANARegistrarController(router, ianaRegistrarService, auth)

	server := httptest.NewServer(router)

	return &TestAPI{
		Router:                  router,
		Server:                  server,
		DB:                      db,
		RoidService:             roidService,
		RegistryOperatorService: registryOperatorService,
		TLDService:              tldService,
		DomainService:           domainService,
		HostService:             hostService,
		RegistrarService:        registrarService,
		ContactService:          contactService,
		PhaseService:            phaseService,
		IANARegistrarService:    ianaRegistrarService,
		AccreditationService:    accreditationService,
		FeeService:              feeService,
		PriceService:            priceService,
		FXService:               fxService,
		NNDNService:             nndnService,
		PremiumListService:      premiumListService,
		PremiumLabelService:     premiumLabelService,
		WhoisService:            whoisService,
	}, nil
}

// newTestDB creates a database connection using env vars with sensible defaults.
// Supports configuration via Doppler or direct env vars.
func newTestDB() (*gorm.DB, error) {
	return postgres.NewConnection(
		postgres.Config{
			User:        envOrDefault("TEST_DB_USER", "postgres"),
			Pass:        envOrDefault("TEST_DB_PASS", "unittest"),
			Host:        envOrDefault("TEST_DB_HOST", "127.0.0.1"),
			Port:        envOrDefault("TEST_DB_PORT", "5432"),
			DBName:      envOrDefault("TEST_DB_NAME", "dos_integration_tests"),
			SSLmode:     envOrDefault("TEST_DB_SSLMODE", "disable"),
			AutoMigrate: true,
		},
	)
}

// envOrDefault returns the value of the environment variable named by the key,
// or the default value if the variable is not set.
func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// --- HTTP Helper Methods ---
// All helpers send real HTTP requests to the httptest.Server, which gives each
// request a fully isolated goroutine lifecycle and eliminates gin.Context pool
// races detected by the -race flag.

// toRecorder converts a real *http.Response into an *httptest.ResponseRecorder
// so existing test assertions (resp.Code, resp.Body) continue to work unchanged.
func toRecorder(resp *http.Response) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	rec.Code = resp.StatusCode
	for k, vs := range resp.Header {
		for _, v := range vs {
			rec.Header().Set(k, v)
		}
	}
	if resp.Body != nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		rec.Body.Write(body)
	}
	return rec
}

// GET performs a GET request to the given path and returns the response recorder.
func (a *TestAPI) GET(path string) *httptest.ResponseRecorder {
	resp, err := http.Get(a.Server.URL + path)
	if err != nil {
		panic(fmt.Sprintf("GET %s failed: %v", path, err))
	}
	return toRecorder(resp)
}

// POST performs a POST request with a JSON body and returns the response recorder.
func (a *TestAPI) POST(path string, body interface{}) *httptest.ResponseRecorder {
	payloadBytes, _ := json.Marshal(body)
	resp, err := http.Post(a.Server.URL+path, "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		panic(fmt.Sprintf("POST %s failed: %v", path, err))
	}
	return toRecorder(resp)
}

// PUT performs a PUT request with a JSON body and returns the response recorder.
func (a *TestAPI) PUT(path string, body interface{}) *httptest.ResponseRecorder {
	payloadBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, a.Server.URL+path, bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(fmt.Sprintf("PUT %s failed: %v", path, err))
	}
	return toRecorder(resp)
}

// DELETE performs a DELETE request and returns the response recorder.
func (a *TestAPI) DELETE(path string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(http.MethodDelete, a.Server.URL+path, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(fmt.Sprintf("DELETE %s failed: %v", path, err))
	}
	return toRecorder(resp)
}

// PATCH performs a PATCH request with a JSON body and returns the response recorder.
func (a *TestAPI) PATCH(path string, body interface{}) *httptest.ResponseRecorder {
	payloadBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPatch, a.Server.URL+path, bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(fmt.Sprintf("PATCH %s failed: %v", path, err))
	}
	return toRecorder(resp)
}

// POSTNoBody performs a POST request without a body and returns the response recorder.
func (a *TestAPI) POSTNoBody(path string) *httptest.ResponseRecorder {
	resp, err := http.Post(a.Server.URL+path, "", nil)
	if err != nil {
		panic(fmt.Sprintf("POSTNoBody %s failed: %v", path, err))
	}
	return toRecorder(resp)
}

// DecodeJSON decodes the JSON response body into the given target.
func DecodeJSON(resp *httptest.ResponseRecorder, target interface{}) error {
	return json.NewDecoder(resp.Body).Decode(target)
}
