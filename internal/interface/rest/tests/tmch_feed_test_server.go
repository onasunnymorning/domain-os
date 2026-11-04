package tests

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

// tmchFeedServer stands in for the Trademark Clearinghouse: it serves a CA
// bundle, an SMD revocation list and a DNL list in the published formats,
// the two lists behind basic auth. Its generation time can be moved
// forward to publish "a newer list".
var tmchFeedServer *httptest.Server

var tmchFeed = struct {
	mu          sync.Mutex
	generatedAt time.Time
	caPEM       []byte
}{generatedAt: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)}

func newTMCHFeedServer() *httptest.Server {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	now := time.Now().UTC()
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "TMCH Test CA"}, NotBefore: now.AddDate(-1, 0, 0), NotAfter: now.AddDate(9, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	tmchFeed.caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tmchFeed.mu.Lock()
		gen := tmchFeed.generatedAt.Format("2006-01-02T15:04:05.0Z")
		tmchFeed.mu.Unlock()
		switch r.URL.Path {
		case "/tmch.crt":
			_, _ = w.Write(tmchFeed.caPEM)
		case "/smdrl/smdrl-latest.csv", "/dnl/dnl-latest.csv":
			user, pass, ok := r.BasicAuth()
			if !ok || user != "registry" || pass != "test-feed" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.URL.Path == "/smdrl/smdrl-latest.csv" {
				_, _ = w.Write([]byte("1," + gen + "\nsmd-id,insertion-datetime\n0000001751373633628125-65535," + gen + "\n"))
				return
			}
			_, _ = w.Write([]byte("1," + gen + "\nDNL,lookup-key,insertion-datetime\nexample-one,2026110400/2/1/5/dkDMTBXNSG4EMHfjzFhsadE9Rzb9lJ-mnHHZxYMFk-cftvfF," + gen + "\nxn--mgbh0fb,2026110400/1/1/1/abc," + gen + "\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// advanceTMCHFeed makes the feed publish lists generated later.
func advanceTMCHFeed(d time.Duration) {
	tmchFeed.mu.Lock()
	defer tmchFeed.mu.Unlock()
	tmchFeed.generatedAt = tmchFeed.generatedAt.Add(d)
}
