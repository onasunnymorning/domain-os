package mosapi

import (
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"slices"
)

// NewHTTPClient returns an HTTP client including a cookie jar
func NewHTTPClient(mc *MosapiConfig) (*http.Client, error) {
	if slices.Contains(SupportedAuthTypes, mc.AuthType) == false {
		return nil, ErrUnsupportedAuthType
	}
	// In case of Basic Auth, just create a cookie jar
	if mc.AuthType == AuthTypeBasic {
		// Create a Cookie Jar -  we only need this for basic auth
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, err
		}
		return &http.Client{
			Jar: jar,
		}, nil
	}
	// Otherwise we're dealing with a certificate
	clientTLSCert, err := tls.LoadX509KeyPair(mc.Certificate, mc.Key)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientTLSCert},
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	return &http.Client{
		Transport: transport,
	}, nil
}
