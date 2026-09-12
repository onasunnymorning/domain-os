package entities

import (
	"strings"

	"errors"
	gonet "github.com/THREATINT/go-net"
)

var (
	ErrInvalidURL = errors.New("invalid url")
)

// URL is our type that represents an URL based on the string type
type URL string

// NewURL normalizes the URL and checks if it is a valid URL and returns an error in case it is not valid.
// If the url is valid it returns an *URL
func NewURL(url string) (*URL, error) {
	u := NormalizeString(url)
	if !gonet.IsURL(u) {
		return nil, ErrInvalidURL
	}
	newURL := URL(u)
	return &newURL, nil
}

// Validate returns a boolean representing the validity of the URL object
func (u *URL) Validate() error {
	s := string(*u)
	if !gonet.IsURL(s) {
		return ErrInvalidURL
	}
	// The domain part is everything between "://" and the first "/" after it.
	//
	// This used to be strings.Split(s, "/")[2], which reads the third field of
	// "scheme://host/path" — and panics on anything with fewer than two
	// slashes. gonet.IsURL accepts plenty of those: "example.com",
	// "example.com/path", "https:/example.com" are all URLs to it. A registrar
	// in a real .co deposit carried one, and because a panic here is a panic
	// through the whole call stack, it ended a 12-million-object escrow
	// validation with nothing to show for it.
	//
	// A URL with no scheme has no domain part to take, so it is invalid here.
	// That is the answer the callers already expect: RDERegistrar.ToEntity
	// reads it as "this may be a bare hostname" and retries with http://
	// prepended.
	scheme, rest, found := strings.Cut(s, "://")
	if !found || scheme == "" || rest == "" {
		return ErrInvalidURL
	}
	host, _, _ := strings.Cut(rest, "/")
	d := DomainName(host)
	if err := d.Validate(); err != nil {
		return err
	}
	return nil
}

// String returns the string value of the URL
func (u *URL) String() string {
	return string(*u)
}
