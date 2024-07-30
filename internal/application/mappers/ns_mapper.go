package mappers

import (
	"fmt"

	"github.com/miekg/dns"
)

// ToDnsNS maps the data we get from our repository (domainname, nameserver) to a dns.NS struct
func ToDnsNS(domainName string, ns string) (dns.RR, error) {
	return dns.NewRR(fmt.Sprintf("%s. 3600 IN NS %s", domainName, ns))
}
