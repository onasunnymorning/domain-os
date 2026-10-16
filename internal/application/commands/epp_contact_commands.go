package commands

// EPPContactAddress is a <contact:addr>.
type EPPContactAddress struct {
	Streets []string
	City    string
	SP      string
	PC      string
	CC      string
}

// EPPPostalInfo is a <contact:postalInfo> of Type "int" or "loc". On
// create every part is given except perhaps Org; in a <contact:chg> any of
// Name, Org and Addr may be absent (nil), and is then left as it is.
type EPPPostalInfo struct {
	Type string
	Name *string
	Org  *string
	Addr *EPPContactAddress
}

// EPPPhone is a <contact:voice> or <contact:fax>: the number, empty if the
// element was, and its extension (the x attribute) if one was given.
type EPPPhone struct {
	Number string
	Ext    *string
}

// EPPDisclose is a <contact:disclose>: Flag true discloses the elements
// listed, false withholds them. Names, Orgs and Addrs list postal info
// types ("int", "loc").
type EPPDisclose struct {
	Flag               bool
	Names, Orgs, Addrs []string
	Voice, Fax, Email  bool
}

// EPPCreateContactCommand is a <contact:create> (RFC 5733 §3.2.1),
// schema-valid but otherwise unchecked. The sponsoring registrar is the
// session's, passed as a scope alongside (ADR 0006).
type EPPCreateContactCommand struct {
	ID         string
	PostalInfo []EPPPostalInfo
	Voice, Fax *EPPPhone
	Email      string
	AuthInfo   string
	Disclose   *EPPDisclose
}

// EPPUpdateContactCommand is a <contact:update> (RFC 5733 §3.2.5): the
// statuses to add and remove, and the values to change. A nil value is
// left as it is.
type EPPUpdateContactCommand struct {
	ID             string
	AddStatuses    []string
	RemoveStatuses []string
	PostalInfo     []EPPPostalInfo
	Voice, Fax     *EPPPhone
	Email          *string
	AuthInfo       *string
	Disclose       *EPPDisclose
}
