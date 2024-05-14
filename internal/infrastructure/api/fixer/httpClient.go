package fixer

import "net/http"

// GetHTTPClient returns an HTTP client that does not follow redirects
func GetHTTPClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}
}
