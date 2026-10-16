package entities

import "errors"

// Refusals of EPP contact commands (RFC 5733). Like the domain refusals,
// they are domain vocabulary (INV-09) that the application layer wraps with
// the detail, and the EPP adapter maps to result codes listed in
// docs/epp/POLICY_REFERENCE.md.
var (
	// ErrEPPPhoneExtension: a voice or fax number with an extension (the x
	// attribute), which this registry does not store.
	ErrEPPPhoneExtension = errors.New("phone extensions are not supported")
	// ErrEPPEmptyPhone: an empty <contact:voice> or <contact:fax> on create;
	// a contact without one leaves the element out.
	ErrEPPEmptyPhone = errors.New("an empty phone number: leave the element out instead")
	// ErrEPPDuplicatePostalInfo: two postal addresses of one type.
	ErrEPPDuplicatePostalInfo = errors.New("at most one postal address of each type")
	// ErrEPPPostalInfoIncomplete: a postal address of a type the contact
	// does not have yet, without a name and an address.
	ErrEPPPostalInfoIncomplete = errors.New("a new postal address needs a name and an address")
	// ErrEPPContactLinked: a delete of a contact a domain uses
	// (RFC 5733 §3.2.2).
	ErrEPPContactLinked = errors.New("the contact is used by a domain")
)
