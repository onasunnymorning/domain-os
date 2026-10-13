package schema

import "regexp"

// Models of epp-1.0.xsd (RFC 5730), eppcom-1.0.xsd and domain-1.0.xsd
// (RFC 5731), for the frames a client sends: <hello> and <command>. Names in
// comments are the XSD type each model mirrors.

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

// Globals are the declarations wildcards may resolve to: the object
// commands the server validates.
var Globals = []*Element{DomainCheck}
