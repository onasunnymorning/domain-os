// eppload drives an EPP server with many sessions, to measure it and to
// rehearse a drop (Phase 7b, docs/epp/RUNBOOK.md § Load and drop tests).
//
// Scenarios:
//
//	check   every session sends <domain:check>s of -check-names names
//	create  every session creates new names under -tld
//	drop    every session races to create the names in -names, from -at,
//	        as a dropcatcher would; the report says who won each and when
//
// The login's password is read from EPP_PASSWORD. The client certificate
// must be pinned for the login (ADR 0014). Run it against a test
// environment, never production: a create run registers domains.
//
// Usage:
//
//	EPP_PASSWORD=... go run ./cmd/tools/eppload -addr localhost:700 \
//	  -cert client.pem -key client-key.pem -login registrar1 \
//	  -scenario check -sessions 8 -duration 1m
package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	if err := cli(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "eppload:", err)
		os.Exit(1)
	}
}

func cli(args []string) error {
	fs := flag.NewFlagSet("eppload", flag.ContinueOnError)
	var (
		o        options
		certFile = fs.String("cert", "", "client certificate PEM, pinned for the login")
		keyFile  = fs.String("key", "", "client key PEM")
		insecure = fs.Bool("insecure", false, "do not verify the server certificate (local development)")
		names    = fs.String("names", "", "drop: file of names to race for, one per line")
		at       = fs.String("at", "", "drop: start time, RFC 3339 (default now)")
		contacts = fs.String("contacts", "", "create, drop: contacts as type=id,type=id (admin, tech, billing)")
		asJSON   = fs.Bool("json", false, "write the report as JSON")
	)
	fs.StringVar(&o.Addr, "addr", "localhost:700", "EPP server host:port")
	fs.StringVar(&o.Login, "login", "", "EPP login")
	fs.StringVar(&o.Scenario, "scenario", "check", "check, create or drop")
	fs.IntVar(&o.Sessions, "sessions", 4, "concurrent sessions")
	fs.DurationVar(&o.Duration, "duration", time.Minute, "how long to run")
	fs.DurationVar(&o.Timeout, "timeout", 30*time.Second, "per frame read and write timeout")
	fs.DurationVar(&o.Interval, "interval", 0, "pause between one session's commands")
	fs.BoolVar(&o.Reconnect, "reconnect", true, "log in again when the server ends a session, as a client would")
	fs.IntVar(&o.CheckNames, "check-names", 50, "check: names per <check>")
	fs.StringVar(&o.TLD, "tld", "test", "check, create: the TLD of generated names")
	fs.StringVar(&o.Create.Registrant, "registrant", "", "create, drop: registrant contact ID")
	fs.StringVar(&o.Create.AuthInfo, "authinfo", "Load-T3st-Xq9#Lm2!vRk7", "create, drop: transfer code")
	if err := fs.Parse(args); err != nil {
		return err
	}
	o.Password = os.Getenv("EPP_PASSWORD")
	switch {
	case o.Login == "" || o.Password == "":
		return fmt.Errorf("-login and EPP_PASSWORD are required")
	case *certFile == "" || *keyFile == "":
		return fmt.Errorf("-cert and -key are required: the server requires a pinned client certificate")
	case o.Sessions < 1:
		return fmt.Errorf("-sessions must be at least 1")
	}
	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		return fmt.Errorf("client certificate: %w", err)
	}
	host, _, err := net.SplitHostPort(o.Addr)
	if err != nil {
		return fmt.Errorf("-addr: %w", err)
	}
	o.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, ServerName: host, MinVersion: tls.VersionTLS12,
		InsecureSkipVerify: *insecure} //nolint:gosec // G402: -insecure is for a local server's self-signed certificate
	o.Create.Contacts = map[string]string{}
	for _, kv := range strings.Split(*contacts, ",") {
		if typ, id, ok := strings.Cut(strings.TrimSpace(kv), "="); ok {
			o.Create.Contacts[typ] = id
		}
	}
	if *names != "" {
		if o.Names, err = readNames(*names); err != nil {
			return err
		}
	}
	o.At = time.Now()
	if *at != "" {
		if o.At, err = time.Parse(time.RFC3339, *at); err != nil {
			return fmt.Errorf("-at: %w", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	rep, err := run(ctx, o)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	rep.print(os.Stdout)
	return nil
}

func readNames(path string) ([]string, error) {
	f, err := os.Open(path) //nolint:gosec // G304: the operator names the file
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if n := strings.TrimSpace(sc.Text()); n != "" && !strings.HasPrefix(n, "#") {
			out = append(out, strings.ToLower(n))
		}
	}
	return out, sc.Err()
}
