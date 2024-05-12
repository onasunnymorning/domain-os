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
	if !gonet.IsURL(string(*u)) {
		return ErrInvalidURL
	}
	// Check if the domain part is valid
	d := DomainName(strings.Split(string(*u), "/")[2])
	if err := d.Validate(); err != nil {
		return err
	}
	return nil
}

// String returns the string value of the URL
func (u *URL) String() string {
	return string(*u)
}
