package queries

import (
	"errors"
	"net"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

const (
	// WhoisQueryTypeDomainName means the string in the query is a valid domain name. This means the query is either a domain name or a nameserver.
	WhoisQueryTypeDomainName = "domainName"
	// WhoisQueryTypeIP means the string in the query is a valid IP address v4 or v6.
	WhoisQueryTypeIP = "ip"
	// WhoisQueryTypeRegistrar means the string in the query not an IP address or a domain name, so it must be a registrar.
	WhoisQueryTypeRegistrar = "registrar"
)

var (
	ErrInvalidWhoisQueryType     = errors.New("invalid WHOIS query type")
	ErrInvalidWhoisQueryEncoding = errors.New("invalid WHOIS query encoding: expecting ASCII only")
	ValidWhoisQueryTypes         = []string{WhoisQueryTypeDomainName, WhoisQueryTypeIP, WhoisQueryTypeRegistrar}
)

// ClassifyWhoisQuery tries to determine determines the type of WHOIS query based on the raw query. It is intended as a first triage for incoming WHOIS requests. This will allow reduction of the database calls if used correctly.
func ClassifyWhoisQuery(rawQuery string) (string, error) {
	// Fail fast if the query is not ASCII.
	if !entities.IsASCII(rawQuery) {
		return "", ErrInvalidWhoisQueryEncoding
	}
	// Check if the query is an IP address first because a v4 ip address can be a valid domain name.
	ip := net.ParseIP(rawQuery)
	if ip != nil {
		return WhoisQueryTypeIP, nil
	}
	// Check if the query is a domain name.
	_, err := entities.NewDomainName(rawQuery)
	if err == nil {
		return WhoisQueryTypeDomainName, nil
	}
	// If not then it must be a registrar
	return WhoisQueryTypeRegistrar, nil
}
