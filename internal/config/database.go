package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
)

// Database locates the registry database and bounds a process's connection
// pool. It is read once at start, in cmd/ (INV-17), and handed to the
// Postgres adapter as its Config: the two structs have the same fields in
// the same order, so a cmd converts with postgres.Config(db). Keep them in
// step.
type Database struct {
	// URL is DATABASE_URL, the whole connection string, when it is set.
	// It takes precedence over the DB_* fields below.
	URL string
	// User, Pass, Host, Port, DBName and SSLmode are the DB_* variables.
	User    string
	Pass    string
	Host    string
	Port    string
	DBName  string
	SSLmode string
	// AutoMigrate is AUTO_MIGRATE; only admin-api reads it, and sets it
	// after loading.
	AutoMigrate bool
	// MaxOpenConns is DB_MAX_OPEN_CONNS: the connections the process may
	// open at once. A quarter are kept idle. The EPP and RDAP nodes have
	// their own setting and do not use this one.
	MaxOpenConns int
}

// DSN is the connection string: URL as given when it is set, otherwise one
// built from the parts, with the credentials percent-escaped so a password
// holding URL-reserved characters survives.
func (d Database) DSN() string {
	if d.URL != "" {
		return d.URL
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(d.User, d.Pass),
		Host:     d.Host + ":" + d.Port,
		Path:     "/" + d.DBName,
		RawQuery: "sslmode=" + url.QueryEscape(d.SSLmode),
	}
	return u.String()
}

const (
	// DefaultDBMaxOpenConns is the pool bound when DB_MAX_OPEN_CONNS is unset.
	DefaultDBMaxOpenConns = 20
	// MinDBMaxOpenConns is the smallest pool that works: a migration holds
	// one connection for its advisory lock while the schema changes run on
	// another, so a pool of one hangs `ryAdminAPI migrate` and
	// AUTO_MIGRATE=true for good.
	MinDBMaxOpenConns = 2
)

// LoadDatabase reads DATABASE_URL, the DB_* variables and DB_MAX_OPEN_CONNS.
// A pool bound that is set but not an integer of at least MinDBMaxOpenConns
// is an error: a pool of one deadlocks the migrator, and an unbounded one is
// what this setting exists to prevent.
func LoadDatabase() (Database, error) {
	db := Database{
		URL:          os.Getenv("DATABASE_URL"),
		User:         valueOr(os.Getenv("DB_USER"), "postgres"),
		Pass:         valueOr(os.Getenv("DB_PASS"), "postgres"),
		Host:         valueOr(os.Getenv("DB_HOST"), "localhost"),
		Port:         valueOr(os.Getenv("DB_PORT"), "5432"),
		DBName:       valueOr(os.Getenv("DB_NAME"), "domain_os"),
		SSLmode:      valueOr(os.Getenv("DB_SSLMODE"), "disable"),
		MaxOpenConns: DefaultDBMaxOpenConns,
	}
	if err := setPositiveInt(&db.MaxOpenConns, "DB_MAX_OPEN_CONNS", os.Getenv("DB_MAX_OPEN_CONNS")); err != nil {
		return Database{}, err
	}
	if db.MaxOpenConns < MinDBMaxOpenConns {
		return Database{}, fmt.Errorf("DB_MAX_OPEN_CONNS=%d is below %d: a migration holds one connection for its lock while the schema changes run on another, so a smaller pool hangs it", db.MaxOpenConns, MinDBMaxOpenConns)
	}
	if db.URL == "" && db.Host == "" {
		return Database{}, errors.New("neither DATABASE_URL nor DB_HOST is set")
	}
	return db, nil
}
