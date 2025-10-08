# EPP Production Architecture & Security Design

## Executive Summary

This document outlines the production architecture for a globally distributed, secure, and resilient EPP service with focus on:
- **Authentication & Authorization** - Client certificate validation, TLSA/DANE, rate limiting
- **Session Management** - Connection pooling, session state, timeout handling
- **Security** - DDoS protection, rate limiting, audit logging
- **Global Distribution** - Anycast deployment for low RTT worldwide
- **Operational Excellence** - Certificate rotation, monitoring, logging

## Current Challenges

1. **Security Threats**
   - ❌ Clients opening many connections (connection exhaustion)
   - ❌ Brute force attacks on login endpoint
   - ❌ No rate limiting per client
   - ❌ No certificate validation/pinning

2. **Certificate Management**
   - ❌ No SSL/TLS certificate rotation strategy
   - ❌ No TLSA/DANE validation
   - ❌ Manual certificate updates

3. **Observability Gaps**
   - ❌ No request/response frame logging
   - ❌ No audit trail
   - ❌ No metrics/monitoring

4. **Global Performance**
   - ❌ Single region deployment
   - ❌ High RTT for distant clients
   - ❌ No geographic load balancing

## Target Architecture

### High-Level Architecture

```
                                    ┌─────────────────┐
                                    │   DNS/Anycast   │
                                    │  (Single IP)    │
                                    └────────┬────────┘
                                             │
                        ┌────────────────────┼────────────────────┐
                        │                    │                    │
                   ┌────▼─────┐        ┌────▼─────┐        ┌────▼─────┐
                   │   US-E    │        │   EU-W   │        │   AP-SE  │
                   │  Region   │        │  Region  │        │  Region  │
                   └────┬─────┘        └────┬─────┘        └────┬─────┘
                        │                    │                    │
              ┌─────────┴─────────┐ ┌────────┴────────┐  ┌────────┴────────┐
              │                   │ │                 │  │                 │
         ┌────▼────┐         ┌────▼────┐         ┌───▼─────┐         ┌────▼────┐
         │ L4 LB   │         │ L4 LB   │         │ L4 LB   │         │ L4 LB   │
         │(HAProxy)│         │(HAProxy)│         │(HAProxy)│         │(HAProxy)│
         └────┬────┘         └────┬────┘         └────┬────┘         └────┬────┘
              │                   │                    │                   │
    ┌─────────┴─────────┐        │           ┌────────┴────────┐         │
    │                   │        │           │                 │         │
┌───▼───┐           ┌───▼───┐┌───▼───┐   ┌───▼───┐       ┌────▼────┐┌───▼───┐
│EPP    │           │EPP    ││EPP    │   │EPP    │       │EPP      ││EPP    │
│Server │           │Server ││Server │   │Server │       │Server   ││Server │
│  #1   │           │  #2   ││  #3   │   │  #4   │       │  #5     ││  #6   │
└───┬───┘           └───┬───┘└───┬───┘   └───┬───┘       └────┬────┘└───┬───┘
    │                   │        │           │                 │        │
    └───────────────────┴────────┴───────────┴─────────────────┴────────┘
                                      │
                        ┌─────────────▼──────────────┐
                        │   Shared Services Layer    │
                        │  - Redis (Session Store)   │
                        │  - PostgreSQL (Registry)   │
                        │  - RabbitMQ (Audit Log)    │
                        │  - Vault (Secrets)         │
                        └────────────────────────────┘
```

### Component Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     EPP Server Process                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌─────────────┐  │
│  │  TLS/DANE    │───▶│   Auth       │───▶│   Session   │  │
│  │  Validator   │    │  Middleware  │    │   Manager   │  │
│  └──────────────┘    └──────────────┘    └─────────────┘  │
│         │                    │                    │        │
│  ┌──────▼────────────────────▼────────────────────▼─────┐  │
│  │            Connection Middleware Layer               │  │
│  │  - Rate Limiter                                      │  │
│  │  - Request Logger                                    │  │
│  │  - Metrics Collector                                 │  │
│  │  - Circuit Breaker                                   │  │
│  └──────────────────────────┬───────────────────────────┘  │
│                             │                              │
│  ┌──────────────────────────▼───────────────────────────┐  │
│  │              EPP Command Router                      │  │
│  │  - Login/Logout                                      │  │
│  │  - Domain Commands                                   │  │
│  │  - Contact Commands                                  │  │
│  │  - Host Commands                                     │  │
│  └──────────────────────────┬───────────────────────────┘  │
│                             │                              │
│  ┌──────────────────────────▼───────────────────────────┐  │
│  │              Business Logic Layer                    │  │
│  │  - Domain Service                                    │  │
│  │  - Contact Service                                   │  │
│  │  - Host Service                                      │  │
│  └──────────────────────────┬───────────────────────────┘  │
│                             │                              │
│  ┌──────────────────────────▼───────────────────────────┐  │
│  │              Data Access Layer                       │  │
│  │  - Repository Pattern                                │  │
│  │  - Database Pool                                     │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## 1. Authentication & Authorization

### 1.1 Multi-Layer Authentication

```go
type AuthenticationChain struct {
    // Layer 1: TLS Client Certificate (REQUIRED)
    CertificateValidator *CertificateValidator
    
    // Layer 2: TLSA/DANE Validation (OPTIONAL but recommended)
    TLSAValidator *TLSAValidator
    
    // Layer 3: EPP Login Credentials
    CredentialValidator *CredentialValidator
    
    // Layer 4: IP Allowlist (OPTIONAL)
    IPValidator *IPValidator
}
```

### 1.2 Certificate-Based Authentication

#### Client Certificate Validation
```go
// internal/infrastructure/epp/auth/cert_validator.go
type CertificateValidator struct {
    // Trusted CA certificates
    TrustedCAs *x509.CertPool
    
    // Certificate revocation list (CRL)
    CRLCache *CRLCache
    
    // OCSP responder
    OCSPClient *ocsp.Client
    
    // Certificate pinning database
    PinnedCerts map[string]*x509.Certificate
}

func (cv *CertificateValidator) ValidateClientCert(
    cert *x509.Certificate,
    registrarID string,
) error {
    // 1. Check certificate validity
    if time.Now().After(cert.NotAfter) {
        return ErrCertificateExpired
    }
    
    // 2. Verify against trusted CA
    if err := cert.CheckSignatureFrom(cv.TrustedCAs); err != nil {
        return ErrUntrustedCA
    }
    
    // 3. Check CRL
    if cv.CRLCache.IsRevoked(cert.SerialNumber) {
        return ErrCertificateRevoked
    }
    
    // 4. OCSP check (optional, can be async)
    if err := cv.OCSPClient.Check(cert); err != nil {
        log.Warn("OCSP check failed", "error", err)
    }
    
    // 5. Certificate pinning (if configured)
    if pinnedCert, exists := cv.PinnedCerts[registrarID]; exists {
        if !cert.Equal(pinnedCert) {
            return ErrCertificateMismatch
        }
    }
    
    return nil
}
```

#### TLSA/DANE Integration
```go
// internal/infrastructure/epp/auth/tlsa_validator.go
type TLSAValidator struct {
    DNSResolver *dns.Resolver
    Cache       *TLSACache
}

// TLSA Record: _700._tcp.epp.example.com
// Format: [Usage] [Selector] [Matching Type] [Certificate Data]
func (tv *TLSAValidator) ValidateTLSA(
    hostname string,
    port int,
    cert *x509.Certificate,
) error {
    // Query TLSA record: _port._tcp.hostname
    tlsaName := fmt.Sprintf("_%d._tcp.%s", port, hostname)
    
    records, err := tv.DNSResolver.LookupTLSA(tlsaName)
    if err != nil {
        return fmt.Errorf("TLSA lookup failed: %w", err)
    }
    
    for _, record := range records {
        if tv.matchTLSARecord(record, cert) {
            return nil
        }
    }
    
    return ErrTLSAValidationFailed
}
```

### 1.3 Rate Limiting & DDoS Protection

```go
// internal/infrastructure/epp/middleware/rate_limiter.go
type RateLimiter struct {
    // Per-IP rate limits
    IPLimiter *redis_rate.Limiter
    
    // Per-registrar limits
    RegistrarLimiter *redis_rate.Limiter
    
    // Connection limits
    MaxConnectionsPerIP    int
    MaxConnectionsPerReg   int
    
    // Failed login tracking
    FailedLoginTracker *FailedLoginTracker
}

type RateLimitConfig struct {
    // Connection limits
    MaxConnPerIP        int           // e.g., 10 connections per IP
    MaxConnPerRegistrar int           // e.g., 100 connections per registrar
    
    // Request rate limits
    RequestsPerSecond   int           // e.g., 100 req/s per registrar
    BurstSize           int           // e.g., 200 burst
    
    // Failed login limits
    MaxFailedLogins     int           // e.g., 5 failed attempts
    LockoutDuration     time.Duration // e.g., 15 minutes
}

func (rl *RateLimiter) CheckConnectionLimit(
    ctx context.Context,
    clientIP string,
    registrarID string,
) error {
    // Check IP-based limit
    ipKey := fmt.Sprintf("conn:ip:%s", clientIP)
    ipConns, err := rl.IPLimiter.GetCount(ctx, ipKey)
    if err != nil {
        return err
    }
    if ipConns >= rl.MaxConnectionsPerIP {
        return ErrTooManyConnections
    }
    
    // Check registrar-based limit
    if registrarID != "" {
        regKey := fmt.Sprintf("conn:reg:%s", registrarID)
        regConns, err := rl.RegistrarLimiter.GetCount(ctx, regKey)
        if err != nil {
            return err
        }
        if regConns >= rl.MaxConnectionsPerReg {
            return ErrTooManyRegistrarConnections
        }
    }
    
    return nil
}

func (rl *RateLimiter) CheckRequestRate(
    ctx context.Context,
    registrarID string,
) error {
    key := fmt.Sprintf("rate:req:%s", registrarID)
    
    res, err := rl.RegistrarLimiter.Allow(ctx, key, redis_rate.PerSecond(
        rl.Config.RequestsPerSecond,
    ))
    if err != nil {
        return err
    }
    
    if res.Allowed == 0 {
        return ErrRateLimitExceeded
    }
    
    return nil
}

// Track failed login attempts
func (rl *RateLimiter) RecordFailedLogin(
    ctx context.Context,
    username string,
    ip string,
) error {
    key := fmt.Sprintf("failed:login:%s:%s", username, ip)
    
    count, err := rl.FailedLoginTracker.Increment(ctx, key)
    if err != nil {
        return err
    }
    
    if count >= rl.Config.MaxFailedLogins {
        // Lock account temporarily
        lockKey := fmt.Sprintf("locked:%s", username)
        rl.FailedLoginTracker.Lock(ctx, lockKey, rl.Config.LockoutDuration)
        return ErrAccountLocked
    }
    
    return nil
}
```

## 2. Session Management

### 2.1 Session Architecture

```go
// internal/domain/session/session.go
type Session struct {
    ID           string
    RegistrarID  string
    ClientIP     string
    CreatedAt    time.Time
    LastActivity time.Time
    ExpiresAt    time.Time
    
    // Authentication state
    Authenticated bool
    Certificate   *x509.Certificate
    
    // Session state
    Language     string
    Version      string
    Namespaces   []string
    
    // Connection tracking
    ConnectionID string
    ServerID     string // For distributed setup
}

type SessionManager struct {
    Store SessionStore // Redis-backed
    TTL   time.Duration
}

// Distributed session store using Redis
type RedisSessionStore struct {
    Client *redis.Client
    Prefix string
}

func (s *RedisSessionStore) Create(
    ctx context.Context,
    session *Session,
) error {
    key := fmt.Sprintf("%s:session:%s", s.Prefix, session.ID)
    
    data, err := json.Marshal(session)
    if err != nil {
        return err
    }
    
    ttl := session.ExpiresAt.Sub(time.Now())
    return s.Client.Set(ctx, key, data, ttl).Err()
}

func (s *RedisSessionStore) Get(
    ctx context.Context,
    sessionID string,
) (*Session, error) {
    key := fmt.Sprintf("%s:session:%s", s.Prefix, sessionID)
    
    data, err := s.Client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return nil, ErrSessionNotFound
    }
    if err != nil {
        return nil, err
    }
    
    var session Session
    if err := json.Unmarshal(data, &session); err != nil {
        return nil, err
    }
    
    return &session, nil
}

func (s *RedisSessionStore) UpdateActivity(
    ctx context.Context,
    sessionID string,
) error {
    session, err := s.Get(ctx, sessionID)
    if err != nil {
        return err
    }
    
    session.LastActivity = time.Now()
    return s.Create(ctx, session)
}
```

### 2.2 Connection Context Enhancement

```go
// internal/infrastructure/epp/server/connection_context.go
type ConnectionContext struct {
    ConnectionID string
    SessionID    string
    RegistrarID  string
    ClientIP     string
    Certificate  *x509.Certificate
    StartTime    time.Time
    
    // Metrics
    RequestCount  int64
    BytesReceived int64
    BytesSent     int64
}

func enrichConnectionContext(
    ctx context.Context,
    conn *tls.Conn,
    sm *SessionManager,
) (context.Context, error) {
    connCtx := &ConnectionContext{
        ConnectionID: generateConnectionID(),
        ClientIP:     conn.RemoteAddr().String(),
        StartTime:    time.Now(),
    }
    
    // Extract client certificate
    if len(conn.ConnectionState().PeerCertificates) > 0 {
        connCtx.Certificate = conn.ConnectionState().PeerCertificates[0]
        
        // Extract registrar ID from certificate
        connCtx.RegistrarID = extractRegistrarID(connCtx.Certificate)
    }
    
    // Create or retrieve session
    session, err := sm.GetOrCreate(ctx, connCtx.RegistrarID, connCtx.ClientIP)
    if err != nil {
        return nil, err
    }
    
    connCtx.SessionID = session.ID
    
    // Add to context
    ctx = context.WithValue(ctx, ContextKeyConnection, connCtx)
    
    return ctx, nil
}
```

## 3. Certificate Management & Rotation

### 3.1 Certificate Rotation Strategy

```go
// internal/infrastructure/epp/certs/rotation.go
type CertificateManager struct {
    CurrentCert  *tls.Certificate
    NextCert     *tls.Certificate
    VaultClient  *vault.Client
    
    // Rotation config
    RotationWindow time.Duration // e.g., 7 days before expiry
    CheckInterval  time.Duration // e.g., 1 hour
}

func (cm *CertificateManager) Start(ctx context.Context) {
    ticker := time.NewTicker(cm.CheckInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if err := cm.checkAndRotate(ctx); err != nil {
                log.Error("Certificate rotation check failed", "error", err)
            }
        case <-ctx.Done():
            return
        }
    }
}

func (cm *CertificateManager) checkAndRotate(ctx context.Context) error {
    // Check if rotation needed
    if !cm.needsRotation() {
        return nil
    }
    
    // Fetch new certificate from Vault
    newCert, err := cm.fetchNewCertificate(ctx)
    if err != nil {
        return fmt.Errorf("failed to fetch new certificate: %w", err)
    }
    
    // Stage new certificate
    cm.NextCert = newCert
    
    // Update server to use new cert (atomic swap)
    cm.CurrentCert = cm.NextCert
    cm.NextCert = nil
    
    log.Info("Certificate rotated successfully",
        "expiry", cm.CurrentCert.Leaf.NotAfter)
    
    return nil
}

func (cm *CertificateManager) GetCertificate(
    clientHello *tls.ClientHelloInfo,
) (*tls.Certificate, error) {
    // Return current certificate
    return cm.CurrentCert, nil
}
```

### 3.2 Vault Integration

```go
// internal/infrastructure/vault/cert_provider.go
type VaultCertProvider struct {
    Client *vault.Client
    PKIPath string // e.g., "pki/issue/epp-server"
}

func (vcp *VaultCertProvider) IssueCertificate(
    ctx context.Context,
    commonName string,
) (*tls.Certificate, error) {
    data := map[string]interface{}{
        "common_name": commonName,
        "ttl":         "8760h", // 1 year
        "alt_names":   "epp.example.com,epp-anycast.example.com",
    }
    
    secret, err := vcp.Client.Logical().Write(vcp.PKIPath, data)
    if err != nil {
        return nil, err
    }
    
    certPEM := secret.Data["certificate"].(string)
    keyPEM := secret.Data["private_key"].(string)
    
    cert, err := tls.X509KeyPair(
        []byte(certPEM),
        []byte(keyPEM),
    )
    if err != nil {
        return nil, err
    }
    
    // Parse certificate for metadata
    cert.Leaf, _ = x509.ParseCertificate(cert.Certificate[0])
    
    return &cert, nil
}
```

## 4. Audit Logging & Observability

### 4.1 Request/Response Frame Logging

```go
// internal/infrastructure/epp/middleware/frame_logger.go
type FrameLogger struct {
    RabbitMQ      *amqp.Channel
    ExchangeName  string
}

type EPPFrame struct {
    Timestamp    time.Time     `json:"timestamp"`
    ConnectionID string        `json:"connection_id"`
    SessionID    string        `json:"session_id"`
    RegistrarID  string        `json:"registrar_id"`
    ClientIP     string        `json:"client_ip"`
    Direction    string        `json:"direction"` // "request" or "response"
    Command      string        `json:"command"`
    Frame        string        `json:"frame"` // Raw XML
    Size         int           `json:"size"`
    Duration     time.Duration `json:"duration_ms,omitempty"`
    ResultCode   string        `json:"result_code,omitempty"`
}

func (fl *FrameLogger) LogRequest(
    ctx context.Context,
    frame string,
) {
    connCtx := ctx.Value(ContextKeyConnection).(*ConnectionContext)
    
    eppFrame := &EPPFrame{
        Timestamp:    time.Now(),
        ConnectionID: connCtx.ConnectionID,
        SessionID:    connCtx.SessionID,
        RegistrarID:  connCtx.RegistrarID,
        ClientIP:     connCtx.ClientIP,
        Direction:    "request",
        Frame:        frame,
        Size:         len(frame),
        Command:      extractCommand(frame),
    }
    
    fl.publishToRabbitMQ(eppFrame)
}

func (fl *FrameLogger) LogResponse(
    ctx context.Context,
    frame string,
    duration time.Duration,
) {
    connCtx := ctx.Value(ContextKeyConnection).(*ConnectionContext)
    
    eppFrame := &EPPFrame{
        Timestamp:    time.Now(),
        ConnectionID: connCtx.ConnectionID,
        SessionID:    connCtx.SessionID,
        RegistrarID:  connCtx.RegistrarID,
        ClientIP:     connCtx.ClientIP,
        Direction:    "response",
        Frame:        frame,
        Size:         len(frame),
        Duration:     duration,
        ResultCode:   extractResultCode(frame),
    }
    
    fl.publishToRabbitMQ(eppFrame)
}

func (fl *FrameLogger) publishToRabbitMQ(frame *EPPFrame) {
    data, _ := json.Marshal(frame)
    
    err := fl.RabbitMQ.Publish(
        fl.ExchangeName, // exchange
        "epp.audit",     // routing key
        false,           // mandatory
        false,           // immediate
        amqp.Publishing{
            ContentType: "application/json",
            Body:        data,
            Timestamp:   frame.Timestamp,
        },
    )
    if err != nil {
        log.Error("Failed to publish to RabbitMQ", "error", err)
    }
}
```

### 4.2 Metrics & Monitoring

```go
// internal/infrastructure/epp/metrics/collector.go
type MetricsCollector struct {
    // Connection metrics
    ActiveConnections   prometheus.Gauge
    TotalConnections    prometheus.Counter
    ConnectionDuration  prometheus.Histogram
    
    // Request metrics
    RequestsTotal       prometheus.Counter
    RequestDuration     prometheus.Histogram
    RequestSize         prometheus.Histogram
    ResponseSize        prometheus.Histogram
    
    // Authentication metrics
    AuthSuccessTotal    prometheus.Counter
    AuthFailureTotal    prometheus.Counter
    
    // Rate limiting metrics
    RateLimitHits       prometheus.Counter
    
    // Error metrics
    ErrorsTotal         prometheus.Counter
}

func (mc *MetricsCollector) Init() {
    mc.ActiveConnections = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "epp_active_connections",
            Help: "Number of active EPP connections",
        },
    )
    
    mc.RequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "epp_requests_total",
            Help: "Total number of EPP requests",
        },
        []string{"command", "result_code", "registrar_id"},
    )
    
    mc.RequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "epp_request_duration_seconds",
            Help:    "EPP request duration",
            Buckets: prometheus.DefBuckets,
        },
        []string{"command"},
    )
    
    // Register all metrics
    prometheus.MustRegister(
        mc.ActiveConnections,
        mc.RequestsTotal,
        mc.RequestDuration,
        // ... other metrics
    )
}
```

## 5. Global Anycast Deployment

### 5.1 Anycast Architecture

**BGP Anycast Setup:**

```yaml
# /etc/bird/bird.conf (on each EPP server)
router id <server-unique-id>;

protocol bgp {
    local as 64512;
    neighbor <upstream-router> as 64511;
    
    export filter {
        # Announce EPP anycast IP
        if net = 203.0.113.100/32 then accept;
        reject;
    };
    
    import none;
}

protocol static {
    route 203.0.113.100/32 via "lo";
}
```

**Health Checks:**

```go
// internal/infrastructure/epp/health/checker.go
type HealthChecker struct {
    Server *epp.Server
}

func (hc *HealthChecker) Handler(w http.ResponseWriter, r *http.Request) {
    // Check server health
    if !hc.Server.IsHealthy() {
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }
    
    // Check dependencies
    checks := []HealthCheck{
        hc.checkDatabase(),
        hc.checkRedis(),
        hc.checkKafka(),
    }
    
    for _, check := range checks {
        if !check.Healthy {
            w.WriteHeader(http.StatusServiceUnavailable)
            json.NewEncoder(w).Encode(check)
            return
        }
    }
    
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
        "region": os.Getenv("REGION"),
    })
}

// Bird BGP health integration
func (hc *HealthChecker) UpdateBGPHealth() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        if hc.Server.IsHealthy() {
            // Announce route
            exec.Command("birdc", "enable", "epp_anycast").Run()
        } else {
            // Withdraw route
            exec.Command("birdc", "disable", "epp_anycast").Run()
        }
    }
}
```

### 5.2 Geographic Distribution

```yaml
# deployment/regions.yaml
regions:
  - name: us-east
    location: Virginia, USA
    anycast_ip: 203.0.113.100
    servers:
      - epp-use1-01.example.com
      - epp-use1-02.example.com
      - epp-use1-03.example.com
    
  - name: eu-west
    location: Ireland
    anycast_ip: 203.0.113.100
    servers:
      - epp-euw1-01.example.com
      - epp-euw1-02.example.com
      - epp-euw1-03.example.com
    
  - name: ap-southeast
    location: Singapore
    anycast_ip: 203.0.113.100
    servers:
      - epp-apse1-01.example.com
      - epp-apse1-02.example.com
      - epp-apse1-03.example.com
```

## 6. Implementation Roadmap

### Phase 1: Security Foundation (Weeks 1-2)
**Priority: CRITICAL**

1. **Certificate-Based Authentication**
   - [ ] Implement `CertificateValidator`
   - [ ] Add certificate pinning support
   - [ ] Integrate CRL checking
   - [ ] Add OCSP validation

2. **Rate Limiting**
   - [ ] Implement Redis-based rate limiter
   - [ ] Add per-IP connection limits
   - [ ] Add per-registrar connection limits
   - [ ] Implement failed login tracking

3. **Session Management**
   - [ ] Create `SessionManager` with Redis backend
   - [ ] Implement session timeout handling
   - [ ] Add session state tracking

**Deliverables:**
- Working certificate validation
- Rate limiting preventing brute force
- Distributed session management

### Phase 2: Observability (Weeks 3-4)
**Priority: HIGH**

1. **Frame Logging**
   - [ ] Implement `FrameLogger` with RabbitMQ
   - [ ] Add request/response capture
   - [ ] Create audit trail pipeline

2. **Metrics & Monitoring**
   - [ ] Add Prometheus metrics
   - [ ] Create Grafana dashboards
   - [ ] Set up alerting (PagerDuty/Opsgenie)

3. **Distributed Tracing**
   - [ ] Add OpenTelemetry instrumentation
   - [ ] Integrate with Jaeger/Tempo

**Deliverables:**
- Complete audit trail
- Real-time metrics dashboards
- Distributed tracing

### Phase 3: Certificate Management (Week 5)
**Priority: HIGH**

1. **Vault Integration**
   - [ ] Set up HashiCorp Vault
   - [ ] Configure PKI secrets engine
   - [ ] Implement `VaultCertProvider`

2. **Certificate Rotation**
   - [ ] Implement `CertificateManager`
   - [ ] Add automated rotation logic
   - [ ] Test zero-downtime rotation

**Deliverables:**
- Automated certificate rotation
- Zero-downtime cert updates

### Phase 4: Global Distribution (Weeks 6-8)
**Priority: MEDIUM**

1. **Anycast Setup**
   - [ ] Configure BGP peering
   - [ ] Set up health checks
   - [ ] Deploy to multiple regions

2. **Load Balancing**
   - [ ] Deploy HAProxy/nginx L4 LB
   - [ ] Configure connection pooling
   - [ ] Add circuit breakers

3. **Data Replication**
   - [ ] Set up PostgreSQL replication
   - [ ] Configure Redis clustering
   - [ ] Implement RabbitMQ multi-region clustering

**Deliverables:**
- Global anycast deployment
- <50ms RTT worldwide
- Multi-region resilience

## 7. Infrastructure as Code

### 7.1 Terraform Configuration

```hcl
# infrastructure/terraform/epp-service/main.tf
module "epp_cluster" {
  source = "./modules/epp-cluster"
  
  for_each = var.regions
  
  region           = each.key
  anycast_ip       = var.anycast_ip
  server_count     = 3
  instance_type    = "c5.2xlarge"
  
  # Networking
  vpc_id           = data.aws_vpc.main[each.key].id
  subnet_ids       = data.aws_subnet_ids.private[each.key].ids
  
  # TLS
  certificate_arn  = aws_acm_certificate.epp[each.key].arn
  
  # Dependencies
  redis_endpoint   = aws_elasticache_cluster.redis[each.key].endpoint
  db_endpoint      = aws_rds_cluster.registry[each.key].endpoint
  kafka_brokers    = aws_msk_cluster.audit[each.key].bootstrap_brokers
  
  tags = {
    Service     = "EPP"
    Environment = var.environment
    Region      = each.key
  }
}
```

### 7.2 Kubernetes Deployment

```yaml
# k8s/epp-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: epp-server
  namespace: registry
spec:
  replicas: 3
  selector:
    matchLabels:
      app: epp-server
  template:
    metadata:
      labels:
        app: epp-server
    spec:
      containers:
      - name: epp-server
        image: registry.example.com/epp-server:latest
        ports:
        - containerPort: 700
          name: epp
          protocol: TCP
        - containerPort: 9090
          name: metrics
        env:
        - name: REGION
          value: "us-east"
        - name: REDIS_ADDR
          valueFrom:
            secretKeyRef:
              name: epp-secrets
              key: redis-addr
        volumeMounts:
        - name: tls-certs
          mountPath: /etc/epp/certs
          readOnly: true
        livenessProbe:
          tcpSocket:
            port: 700
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 9090
          initialDelaySeconds: 10
          periodSeconds: 5
      volumes:
      - name: tls-certs
        secret:
          secretName: epp-tls-certs
---
apiVersion: v1
kind: Service
metadata:
  name: epp-server
  namespace: registry
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
spec:
  type: LoadBalancer
  loadBalancerIP: 203.0.113.100  # Anycast IP
  ports:
  - port: 700
    targetPort: 700
    protocol: TCP
  selector:
    app: epp-server
```

## 8. Recommended Starting Point

### **Start Here: Phase 1 - Security Foundation**

#### Step 1: Certificate-Based Authentication (Week 1)
**File:** `internal/infrastructure/epp/auth/cert_validator.go`

```go
package auth

import (
    "crypto/x509"
    "fmt"
    "time"
)

type CertificateValidator struct {
    TrustedCAs     *x509.CertPool
    RequireClientCert bool
}

func NewCertificateValidator(caCertPEM []byte) (*CertificateValidator, error) {
    caPool := x509.NewCertPool()
    if !caPool.AppendCertsFromPEM(caCertPEM) {
        return nil, fmt.Errorf("failed to parse CA certificate")
    }
    
    return &CertificateValidator{
        TrustedCAs:        caPool,
        RequireClientCert: true,
    }, nil
}

func (cv *CertificateValidator) Validate(cert *x509.Certificate) error {
    // Implementation here
}
```

#### Step 2: Rate Limiter (Week 1-2)
**File:** `internal/infrastructure/epp/middleware/rate_limiter.go`

Start with Redis-based rate limiting using `github.com/go-redis/redis_rate`.

#### Step 3: Session Manager (Week 2)
**File:** `internal/domain/session/manager.go`

Implement Redis-backed session storage.

### Quick Wins

1. **Today:** Add connection counting to prevent exhaustion
2. **This Week:** Implement basic rate limiting
3. **Next Week:** Add audit logging to Kafka
4. **Month 1:** Deploy to multiple regions with anycast

## Summary

This architecture provides:
- ✅ **Security**: Multi-layer auth, rate limiting, DDoS protection
- ✅ **Scalability**: Anycast, load balancing, distributed sessions
- ✅ **Observability**: Full audit trail, metrics, tracing
- ✅ **Reliability**: Certificate rotation, health checks, multi-region
- ✅ **Performance**: <50ms RTT globally via anycast

**Next Action:** Start with Phase 1, implementing certificate validation and rate limiting to address immediate security concerns.
