package schema

import "regexp"

// Models of epp-1.0.xsd (RFC 5730), eppcom-1.0.xsd, domain-1.0.xsd
// (RFC 5731) and host-1.0.xsd (RFC 5732), for the frames a client sends:
// <hello> and <command>. Names in comments are the XSD type each model
// mirrors.

func token(minLen, maxLen int) *Simple {
	return &Simple{Base: Token, MinLen: minLen, MaxLen: maxLen}
}

var (
	// eppcom:clIDType
	clIDType = token(3, 16)
	// eppcom:labelType
	labelType = token(1, 255)
	// epp:pwType
	pwType = token(6, 16)
	// epp:trIDStringType
	trIDStringType = token(3, 64)
	// epp:versionType
	versionType = &Simple{Base: Token, Pattern: regexp.MustCompile(`^[1-9]+\.[0-9]+$`), Enum: []string{"1.0"}}
	anyURI      = &Simple{Base: AnyURI}
	language    = &Simple{Base: Language}
)

// extAnyType is epp:extAnyType: one or more elements from other namespaces.
func extAnyType() Seq {
	return Seq{Any{OtherThan: NSEPP, Max: Unbounded}}
}

// readWriteType is epp:readWriteType: exactly one object element.
func readWriteType() Seq { return Seq{Any{OtherThan: NSEPP}} }

// loginType is epp:loginType.
var login = &Element{NS: NSEPP, Name: "login", Content: Seq{
	&Element{NS: NSEPP, Name: "clID", Simple: clIDType},
	&Element{NS: NSEPP, Name: "pw", Simple: pwType},
	&Element{NS: NSEPP, Name: "newPW", Min: Optional, Simple: pwType},
	&Element{NS: NSEPP, Name: "options", Content: Seq{ // epp:credsOptionsType
		&Element{NS: NSEPP, Name: "version", Simple: versionType},
		&Element{NS: NSEPP, Name: "lang", Simple: language},
	}},
	&Element{NS: NSEPP, Name: "svcs", Content: Seq{ // epp:loginSvcType
		&Element{NS: NSEPP, Name: "objURI", Max: Unbounded, Simple: anyURI},
		&Element{NS: NSEPP, Name: "svcExtension", Min: Optional, Content: Seq{ // epp:extURIType
			&Element{NS: NSEPP, Name: "extURI", Max: Unbounded, Simple: anyURI},
		}},
	}},
}}

var (
	pollOp     = &Simple{Base: Token, Enum: []string{"ack", "req"}}
	transferOp = &Simple{Base: Token, Enum: []string{"approve", "cancel", "query", "reject", "request"}}
)

// command is epp:commandType.
var command = &Element{NS: NSEPP, Name: "command", Content: Seq{
	Choice{Alts: []Particle{
		&Element{NS: NSEPP, Name: "check", Content: readWriteType()},
		&Element{NS: NSEPP, Name: "create", Content: readWriteType()},
		&Element{NS: NSEPP, Name: "delete", Content: readWriteType()},
		&Element{NS: NSEPP, Name: "info", Content: readWriteType()},
		login,
		&Element{NS: NSEPP, Name: "logout", AnyType: true},
		&Element{NS: NSEPP, Name: "poll", Attrs: []Attribute{ // epp:pollType
			{Name: "op", Type: pollOp, Required: true},
			{Name: "msgID", Type: &Simple{Base: Token}},
		}},
		&Element{NS: NSEPP, Name: "renew", Content: readWriteType()},
		&Element{NS: NSEPP, Name: "transfer", Content: readWriteType(), Attrs: []Attribute{ // epp:transferType
			{Name: "op", Type: transferOp, Required: true},
		}},
		&Element{NS: NSEPP, Name: "update", Content: readWriteType()},
	}},
	&Element{NS: NSEPP, Name: "extension", Min: Optional, Content: extAnyType()},
	&Element{NS: NSEPP, Name: "clTRID", Min: Optional, Simple: trIDStringType},
}}

// Root is the <epp> element as a client sends it: a <hello> or a
// <command>. epp:eppType also allows <greeting>, <response> and
// <extension>, which only a server sends; the dispatcher answers those
// before validation.
var Root = &Element{NS: NSEPP, Name: "epp", Content: Choice{Alts: []Particle{
	&Element{NS: NSEPP, Name: "hello", AnyType: true},
	command,
}}}

// DomainCheck is <domain:check>, domain:mNameType.
var DomainCheck = &Element{NS: NSDomain, Name: "check", Content: Seq{
	&Element{NS: NSDomain, Name: "name", Max: Unbounded, Simple: labelType},
}}

// authInfoType is domain:authInfoType, and contact:authInfoType in ns: a
// password (eppcom:pwAuthInfoType), or an extension
// (eppcom:extAuthInfoType, one element outside eppcom).
func authInfoType(ns string, min int) *Element {
	return &Element{NS: ns, Name: "authInfo", Min: min, Content: Choice{Alts: []Particle{
		&Element{NS: ns, Name: "pw", Simple: &Simple{Base: NormalizedString}, Attrs: []Attribute{
			{Name: "roid", Type: roidType},
		}},
		&Element{NS: ns, Name: "ext", Content: Seq{Any{OtherThan: NSEPPCom}}},
	}}}
}

var (
	// eppcom:roidType. XSD's \w is every character but punctuation,
	// separators and others.
	roidType = &Simple{Base: Token, Pattern: regexp.MustCompile(`^([^\p{P}\p{Z}\p{C}]|_){1,80}-[^\p{P}\p{Z}\p{C}]{1,8}$`)}
	// domain:hostsType
	hostsType = &Simple{Base: Token, Enum: []string{"all", "del", "none", "sub"}}
	// domain:pLimitType
	pLimitType = &Simple{Base: UnsignedShort, MinInclusive: 1, MaxInclusive: 99, HasRange: true}
	// domain:pUnitType
	pUnitType = &Simple{Base: Token, Enum: []string{"y", "m"}}
	// domain:contactAttrType
	contactAttrType = &Simple{Base: Token, Enum: []string{"admin", "billing", "tech"}}
	// host:addrStringType and host:ipType
	addrStringType = token(3, 45)
	ipType         = &Simple{Base: Token, Enum: []string{"v4", "v6"}}
)

// DomainInfo is <domain:info>, domain:infoType.
var DomainInfo = &Element{NS: NSDomain, Name: "info", Content: Seq{
	&Element{NS: NSDomain, Name: "name", Simple: labelType, Attrs: []Attribute{{Name: "hosts", Type: hostsType}}},
	authInfoType(NSDomain, Optional),
}}

// DomainCreate is <domain:create>, domain:createType.
var DomainCreate = &Element{NS: NSDomain, Name: "create", Content: Seq{
	&Element{NS: NSDomain, Name: "name", Simple: labelType},
	&Element{NS: NSDomain, Name: "period", Min: Optional, Simple: pLimitType, Attrs: []Attribute{
		{Name: "unit", Type: pUnitType, Required: true},
	}},
	domainNS(),
	&Element{NS: NSDomain, Name: "registrant", Min: Optional, Simple: clIDType},
	&Element{NS: NSDomain, Name: "contact", Min: Optional, Max: Unbounded, Simple: clIDType, Attrs: []Attribute{
		{Name: "type", Type: contactAttrType},
	}},
	authInfoType(NSDomain, 0),
}}

// domainNS is domain:nsType, minOccurs 0.
func domainNS() *Element {
	return &Element{NS: NSDomain, Name: "ns", Min: Optional, Content: Choice{Alts: []Particle{
		&Element{NS: NSDomain, Name: "hostObj", Max: Unbounded, Simple: labelType},
		&Element{NS: NSDomain, Name: "hostAttr", Max: Unbounded, Content: Seq{ // domain:hostAttrType
			&Element{NS: NSDomain, Name: "hostName", Simple: labelType},
			&Element{NS: NSDomain, Name: "hostAddr", Min: Optional, Max: Unbounded, Simple: addrStringType, Attrs: []Attribute{
				{Name: "ip", Type: ipType},
			}},
		}},
	}}}
}

var (
	// domain:statusValueType
	domainStatusValueType = &Simple{Base: Token, Enum: []string{
		"clientDeleteProhibited", "clientHold", "clientRenewProhibited", "clientTransferProhibited", "clientUpdateProhibited",
		"inactive", "ok", "pendingCreate", "pendingDelete", "pendingRenew", "pendingTransfer", "pendingUpdate",
		"serverDeleteProhibited", "serverHold", "serverRenewProhibited", "serverTransferProhibited", "serverUpdateProhibited",
	}}
	// domain:clIDChgType: a clIDType that may be empty, to remove the
	// registrant.
	clIDChgType = token(0, 16)
)

// domainAddRem is domain:addRemType, minOccurs 0.
func domainAddRem(name string) *Element {
	return &Element{NS: NSDomain, Name: name, Min: Optional, Content: Seq{
		domainNS(),
		&Element{NS: NSDomain, Name: "contact", Min: Optional, Max: Unbounded, Simple: clIDType, Attrs: []Attribute{
			{Name: "type", Type: contactAttrType},
		}},
		&Element{NS: NSDomain, Name: "status", Min: Optional, Max: 11, Simple: &Simple{Base: NormalizedString}, Attrs: []Attribute{ // domain:statusType
			{Name: "s", Type: domainStatusValueType, Required: true},
			{Name: "lang", Type: language},
		}},
	}}
}

// DomainUpdate is <domain:update>, domain:updateType.
var DomainUpdate = &Element{NS: NSDomain, Name: "update", Content: Seq{
	&Element{NS: NSDomain, Name: "name", Simple: labelType},
	domainAddRem("add"),
	domainAddRem("rem"),
	&Element{NS: NSDomain, Name: "chg", Min: Optional, Content: Seq{ // domain:chgType
		&Element{NS: NSDomain, Name: "registrant", Min: Optional, Simple: clIDChgType},
		&Element{NS: NSDomain, Name: "authInfo", Min: Optional, Content: Choice{Alts: []Particle{ // domain:authInfoChgType
			&Element{NS: NSDomain, Name: "pw", Simple: &Simple{Base: NormalizedString}, Attrs: []Attribute{{Name: "roid", Type: roidType}}},
			&Element{NS: NSDomain, Name: "ext", Content: Seq{Any{OtherThan: NSEPPCom}}},
			// Declared without a type: xs:anyType.
			&Element{NS: NSDomain, Name: "null", AnyType: true},
		}}},
	}},
}}

// DomainRenew is <domain:renew>, domain:renewType.
var DomainRenew = &Element{NS: NSDomain, Name: "renew", Content: Seq{
	&Element{NS: NSDomain, Name: "name", Simple: labelType},
	&Element{NS: NSDomain, Name: "curExpDate", Simple: &Simple{Base: Date}},
	&Element{NS: NSDomain, Name: "period", Min: Optional, Simple: pLimitType, Attrs: []Attribute{
		{Name: "unit", Type: pUnitType, Required: true},
	}},
}}

// DomainDelete is <domain:delete>, domain:sNameType.
var DomainDelete = &Element{NS: NSDomain, Name: "delete", Content: Seq{
	&Element{NS: NSDomain, Name: "name", Simple: labelType},
}}

// Models of secDNS-1.1.xsd (RFC 5910) for <secDNS:create> and
// <secDNS:update>.
var (
	unsignedShort = &Simple{Base: UnsignedShort}
	unsignedByte  = &Simple{Base: UnsignedByte}
	// secDNS:maxSigLifeType
	maxSigLifeType = &Simple{Base: Int, MinInclusive: 1, MaxInclusive: 2147483647, HasRange: true}
	// secDNS:keyType
	keyType = &Simple{Base: Base64Binary, MinLen: 1}
)

// secDNSKeyData is secDNS:keyDataType.
func secDNSKeyData(min, max int) *Element {
	return &Element{NS: NSSecDNS, Name: "keyData", Min: min, Max: max, Content: Seq{
		&Element{NS: NSSecDNS, Name: "flags", Simple: unsignedShort},
		&Element{NS: NSSecDNS, Name: "protocol", Simple: unsignedByte},
		&Element{NS: NSSecDNS, Name: "alg", Simple: unsignedByte},
		&Element{NS: NSSecDNS, Name: "pubKey", Simple: keyType},
	}}
}

// secDNSDSData is secDNS:dsDataType, maxOccurs unbounded.
func secDNSDSData() *Element {
	return &Element{NS: NSSecDNS, Name: "dsData", Max: Unbounded, Content: Seq{
		&Element{NS: NSSecDNS, Name: "keyTag", Simple: unsignedShort},
		&Element{NS: NSSecDNS, Name: "alg", Simple: unsignedByte},
		&Element{NS: NSSecDNS, Name: "digestType", Simple: unsignedByte},
		&Element{NS: NSSecDNS, Name: "digest", Simple: &Simple{Base: HexBinary}},
		secDNSKeyData(Optional, 0),
	}}
}

// secDNSDSOrKey is secDNS:dsOrKeyType, as the element name, minOccurs min.
func secDNSDSOrKey(name string, min int) *Element {
	return &Element{NS: NSSecDNS, Name: name, Min: min, Content: Seq{
		&Element{NS: NSSecDNS, Name: "maxSigLife", Min: Optional, Simple: maxSigLifeType},
		Choice{Alts: []Particle{secDNSDSData(), secDNSKeyData(0, Unbounded)}},
	}}
}

// SecDNSCreate is <secDNS:create>, secDNS:dsOrKeyType.
var SecDNSCreate = secDNSDSOrKey("create", 0)

// SecDNSUpdate is <secDNS:update>, secDNS:updateType.
var SecDNSUpdate = &Element{NS: NSSecDNS, Name: "update", Attrs: []Attribute{{Name: "urgent", Type: boolean}}, Content: Seq{
	&Element{NS: NSSecDNS, Name: "rem", Min: Optional, Content: Choice{Alts: []Particle{ // secDNS:remType
		&Element{NS: NSSecDNS, Name: "all", Simple: boolean},
		secDNSDSData(),
		secDNSKeyData(0, Unbounded),
	}}},
	secDNSDSOrKey("add", Optional),
	&Element{NS: NSSecDNS, Name: "chg", Min: Optional, Content: Seq{ // secDNS:chgType
		&Element{NS: NSSecDNS, Name: "maxSigLife", Min: Optional, Simple: maxSigLifeType},
	}},
}}

// Models of host-1.0.xsd (RFC 5732) for the host commands.
var (
	// host:statusValueType
	hostStatusValueType = &Simple{Base: Token, Enum: []string{
		"clientDeleteProhibited", "clientUpdateProhibited", "linked", "ok", "pendingCreate",
		"pendingDelete", "pendingTransfer", "pendingUpdate", "serverDeleteProhibited", "serverUpdateProhibited",
	}}
)

func hostName() *Element { return &Element{NS: NSHost, Name: "name", Simple: labelType} }

// hostAddrs is host:addrType, minOccurs 0 and maxOccurs unbounded.
func hostAddrs() *Element {
	return &Element{NS: NSHost, Name: "addr", Min: Optional, Max: Unbounded, Simple: addrStringType, Attrs: []Attribute{
		{Name: "ip", Type: ipType},
	}}
}

// hostAddRem is host:addRemType.
func hostAddRem(name string) *Element {
	return &Element{NS: NSHost, Name: name, Min: Optional, Content: Seq{
		hostAddrs(),
		&Element{NS: NSHost, Name: "status", Min: Optional, Max: 7, Simple: &Simple{Base: NormalizedString}, Attrs: []Attribute{ // host:statusType
			{Name: "s", Type: hostStatusValueType, Required: true},
			{Name: "lang", Type: language},
		}},
	}}
}

// HostCheck is <host:check>, host:mNameType.
var HostCheck = &Element{NS: NSHost, Name: "check", Content: Seq{
	&Element{NS: NSHost, Name: "name", Max: Unbounded, Simple: labelType},
}}

// HostInfo is <host:info>, host:sNameType.
var HostInfo = &Element{NS: NSHost, Name: "info", Content: Seq{hostName()}}

// HostDelete is <host:delete>, host:sNameType.
var HostDelete = &Element{NS: NSHost, Name: "delete", Content: Seq{hostName()}}

// HostCreate is <host:create>, host:createType.
var HostCreate = &Element{NS: NSHost, Name: "create", Content: Seq{hostName(), hostAddrs()}}

// HostUpdate is <host:update>, host:updateType.
var HostUpdate = &Element{NS: NSHost, Name: "update", Content: Seq{
	hostName(),
	hostAddRem("add"),
	hostAddRem("rem"),
	&Element{NS: NSHost, Name: "chg", Min: Optional, Content: Seq{hostName()}}, // host:chgType
}}

// Globals are the declarations wildcards may resolve to: the object
// commands and extensions the server validates.
var Globals = []*Element{
	DomainCheck, DomainInfo, DomainCreate, DomainUpdate, DomainRenew, DomainDelete,
	HostCheck, HostInfo, HostCreate, HostUpdate, HostDelete,
	ContactCheck, ContactInfo, ContactCreate, ContactUpdate, ContactDelete,
	SecDNSCreate, SecDNSUpdate,
}
