package rdevalidate

import (
	"errors"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// unnamedRule is what a rejection reduces to when nothing in entityRules
// matches it. It says nothing about the object, which is the point: the
// error's own text is not safe to repeat, so an unrecognised rejection has to
// stay anonymous until someone adds the rule to the table.
const unnamedRule = "refused by a registry rule this validator cannot name yet"

// entityRules maps the errors an RDE object's ToEntity returns to the rule
// each one stands for.
//
// The error is never echoed. Entity constructors build some of their errors
// with fmt.Errorf and the offending value — rdeDomain.go puts the domain name
// in one, and time.Parse repeats the field's raw text in every error it
// returns — while a finding may carry constant templates and numbers only. So
// a rejection is matched against this table and the table's own constant text
// is what reaches the report.
//
// Order matters. errors.Join and %w chains can match more than one entry and
// the first match wins, so a specific rule precedes the general one it is a
// case of.
//
// A rule names a field only where the sentinel is specific to one. Several
// value objects are shared: URL.Validate hands back DomainName.Validate's own
// error unchanged (url.go), so a bad registrar URL and a bad domain name are
// the same error here and neither may claim the other's field.
var entityRules = []struct {
	err  error
	rule string
}{
	// roid. The likeliest reason a deposit produced by another registry system
	// is refused wholesale: domain-os requires its own <id>_<OBJECT>-<repo>
	// shape, which a deposit written as "D123-EXAMPLE" does not have.
	{entities.ErrInvalidDomainRoID, "roid: the object identifier between _ and - must be " + entities.DOMAIN_ROID_ID},
	{entities.ErrInvalidContactRoID, "roid: the object identifier between _ and - must be " + entities.CONTACT_ROID_ID},
	{entities.ErrInvalidHostRoID, "roid: the object identifier between _ and - must be " + entities.HOST_ROID_ID},
	{entities.ErrInvalidRoid, "roid: must have the form <id>_<OBJECT>-<repository>, e.g. 1_" +
		entities.DOMAIN_ROID_ID + "-" + entities.EPP_REPOSITORY_ID},

	// hostnames. Checked on a domain or host name, on a domain's nameservers
	// and on the host part of a registrar url, and the error is the same in
	// every case, so these name the rule and not the field.
	{entities.ErrTLDAsDomain, "a domain name is the TLD itself, not a name under it"},
	{entities.ErrEmptyDomainName, "a hostname field is empty"},
	{entities.ErrInvalidLabelLength, "a hostname label must be 1 to 63 characters"},
	{entities.ErrInvalidLabelDash, "a hostname label may not start or end with a hyphen"},
	{entities.ErrInvalidLabelDoubleDash, "a non-IDN hostname label may not contain two consecutive hyphens"},
	{entities.ErrInvalidLabelIDN, "an xn-- label must decode to Unicode"},
	{entities.ErrLabelContainsInvalidCharacter, "a hostname contains a character not allowed in one"},
	{entities.ErrinvalIdDomainNameLength, "a hostname is longer than a domain name may be"},
	{entities.ErrInvalidDomainName, "a hostname is not a valid domain name"},

	// IDN pairing between name, uName and originalName.
	{entities.ErrNoUNameProvidedForIDNDomain, "uName: an IDN domain must carry its Unicode form"},
	{entities.ErrUNameDoesNotMatchDomain, "uName: is not the Unicode form of name"},
	{entities.ErrUNameFieldReservedForIDNDomains, "uName: set on a domain that is not an IDN"},
	{entities.ErrOriginalNameShouldBeAlabel, "originalName: must be an A-label"},
	{entities.ErrOriginalNameEqualToDomain, "originalName: must differ from name"},
	{entities.ErrOriginalNameFieldReservedForIDN, "originalName: set on a domain that is not an IDN"},

	// registrar and contact identifiers.
	{entities.ErrEmptyClientID, "clID: is empty"},
	{entities.ErrInvalidRegistrarClID, "clID: is not a valid registrar identifier"},
	{entities.ErrInvalidClIDType, "clID, crRr, upRr, registrant or contact: is not a valid EPP client identifier (3 to 16 characters)"},
	{entities.ErrInvalidContact, "contact: type attribute must be admin, tech or billing"},

	// statuses.
	{entities.ErrInvalidDomainStatusCombination, "status: the statuses on this domain cannot be held at once"},
	{entities.ErrInvalidDomainStatus, "status: is not an EPP domain status"},
	{entities.ErrInvalidContactStatusCombination, "status: the statuses on this contact cannot be held at once"},
	{entities.ErrInvalidContactStatus, "status: is not an EPP contact status"},
	{entities.ErrInvalidRegistrarStatus, "status: is not a registrar status"},

	// postal information.
	{entities.ErrInvalidPostalInfoCount, "postalInfo: a contact may carry at most one int and one loc form"},
	{entities.ErrInvalidPostalInfoEnumType, "postalInfo: type attribute must be int or loc"},
	{entities.ErrInvalidASCIIInIntAddress, "postalInfo: an int address must be ASCII only"},
	{entities.ErrInvalidStreetCount, "postalInfo: at most three street lines are allowed"},
	{entities.ErrInvalidStreet, "postalInfo: a street line is empty or too long"},
	{entities.ErrInvalidCity, "postalInfo: city is empty or too long"},
	{entities.ErrInvalidStateProvince, "postalInfo: sp is too long"},
	{entities.ErrInvalidPostalCode, "postalInfo: pc is not a valid postal code"},
	{entities.ErrInvalidPCType, "postalInfo: pc is not a valid postal code"},
	{entities.ErrInvalidCountryCode, "postalInfo: cc is not an ISO 3166-1 alpha-2 country code"},
	{entities.ErrInvalidOptPostalLineType, "postalInfo: an optional line exceeds its maximum length"},
	{entities.ErrInvalidPostalLineType, "postalInfo: a required line is empty or exceeds its maximum length"},
	{entities.ErrInvalidContactPostalInfo, "postalInfo: is not a valid contact postalInfo"},
	{entities.ErrInvalidRegistrarPostalInfo, "postalInfo: is not a valid registrar postalInfo"},

	// contactable details.
	{entities.ErrInvalidEmail, "email: is not a valid address"},
	{entities.ErrInvalidE164Type, "voice or fax: is not an E.164 number"},
	{entities.ErrRegistrarMissingEmail, "email: a registrar must have one"},
	{entities.ErrRegistrarMissingName, "name: a registrar must have one"},
	{entities.ErrInvalidURL, "url: is not a valid URL"},
	{entities.ErrInvalidRegistrarIANAStatus, "gurid: is not consistent with the registrar's status"},

	// hosts.
	{entities.ErrInvalidIP, "addr: is not a valid IP address"},
	{entities.ErrInBailiwickHostsMustHaveAddress, "addr: an in-bailiwick host must carry at least one"},

	// timestamps that are not a plain parse failure.
	{entities.ErrTimeStampNotUTC, "a date field is not in UTC"},
	{entities.ErrInvalidTimeFormat, "a date field is not an RFC 3339 timestamp"},

	// the object-level catch-alls, last: every rule above is a case of one.
	{entities.ErrInvalidDomain, "the assembled domain failed its final validation"},
	{entities.ErrInvalidContactPostalInfo, "the assembled contact failed its final validation"},
	{entities.ErrInvalidRegistrar, "the assembled registrar failed its final validation"},
	{entities.ErrInvalidNNDN, "the assembled NNDN failed its final validation"},
}

// entityRule names the rule that refused an otherwise well-formed RDE object.
//
// dates is a name/value list of the object's date elements, in the RDE
// element names an operator would search the deposit for. It is used only to
// attribute a *time.ParseError, whose own message repeats the offending text
// and so can never be reported: the first date element that will not parse is
// named instead.
func entityRule(err error, dates ...string) string {
	if err == nil {
		return ""
	}
	var pe *time.ParseError
	if errors.As(err, &pe) {
		if name := firstUnparsableDate(dates...); name != "" {
			return name + ": is not an RFC 3339 timestamp"
		}
		return "a date field is not an RFC 3339 timestamp"
	}
	for _, e := range entityRules {
		if errors.Is(err, e.err) {
			return e.rule
		}
	}
	return unnamedRule
}

// firstUnparsableDate returns the name of the first name/value pair whose
// value is not an RFC 3339 timestamp. An empty value counts: the entity
// constructors parse several date elements unconditionally, so a deposit that
// omits one is refused exactly as if it had written one wrong.
func firstUnparsableDate(pairs ...string) string {
	for i := 0; i+1 < len(pairs); i += 2 {
		v := strings.TrimSpace(pairs[i+1])
		if _, err := time.Parse(time.RFC3339, v); err != nil {
			return pairs[i]
		}
	}
	return ""
}
