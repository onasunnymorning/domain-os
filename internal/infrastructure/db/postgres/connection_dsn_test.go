package postgres

import (
	"net/url"
	"testing"
)

// TestBuildDSN_EscapesReservedCharacters guards against the runtime bug where a
// database password containing URL-reserved characters (routine in RDS- and
// Secrets Manager-generated credentials) was interpolated raw into a
// postgres:// URL, so the password bled into the host:port and connection
// setup panicked on startup with "invalid port ... after host".
func TestBuildDSN_EscapesReservedCharacters(t *testing.T) {
	cfg := Config{
		User:    "alpaca_admin",
		Pass:    `p@ss:w/rd?#`,
		Host:    "db.internal.example.com",
		Port:    "5432",
		DBName:  "alpaca",
		SSLmode: "require",
	}

	dsn := buildDSN(cfg)

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("buildDSN produced an unparseable URL %q: %v", dsn, err)
	}
	if got := u.Port(); got != cfg.Port {
		t.Errorf("port = %q, want %q — the password likely bled into host:port", got, cfg.Port)
	}
	if got := u.Hostname(); got != cfg.Host {
		t.Errorf("host = %q, want %q", got, cfg.Host)
	}
	if got := u.User.Username(); got != cfg.User {
		t.Errorf("user = %q, want %q", got, cfg.User)
	}
	gotPass, _ := u.User.Password()
	if gotPass != cfg.Pass {
		t.Errorf("password round-trip = %q, want %q", gotPass, cfg.Pass)
	}
	if got := u.Query().Get("sslmode"); got != cfg.SSLmode {
		t.Errorf("sslmode = %q, want %q", got, cfg.SSLmode)
	}
}
