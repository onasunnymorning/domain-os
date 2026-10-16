package schema

import "regexp"

// Models of contact-1.0.xsd (RFC 5733) for the contact commands. Names in
// comments are the XSD type each model mirrors.

var (
	// contact:ccType
	ccType = &Simple{Base: Token, MinLen: 2, MaxLen: 2}
	// contact:e164StringType
	e164StringType = &Simple{Base: Token, MaxLen: 17, Pattern: regexp.MustCompile(`^(\+[0-9]{1,3}\.[0-9]{1,14})?$`)}
	// contact:pcType
	pcType = &Simple{Base: Token, MaxLen: 16}
	// contact:postalLineType
	postalLineType = &Simple{Base: NormalizedString, MinLen: 1, MaxLen: 255}
	// contact:optPostalLineType
	optPostalLineType = &Simple{Base: NormalizedString, MaxLen: 255}
	// contact:postalInfoEnumType
	postalInfoEnumType = &Simple{Base: Token, Enum: []string{"loc", "int"}}
	// eppcom:minTokenType
	minTokenType = &Simple{Base: Token, MinLen: 1}
	// contact:statusValueType
	contactStatusValueType = &Simple{Base: Token, Enum: []string{
		"clientDeleteProhibited", "clientTransferProhibited", "clientUpdateProhibited", "linked", "ok",
		"pendingCreate", "pendingDelete", "pendingTransfer", "pendingUpdate",
		"serverDeleteProhibited", "serverTransferProhibited", "serverUpdateProhibited",
	}}
	boolean = &Simple{Base: Boolean}
)

func contactID() *Element { return &Element{NS: NSContact, Name: "id", Simple: clIDType} }

// contactE164 is contact:e164Type, minOccurs 0.
func contactE164(name string) *Element {
	return &Element{NS: NSContact, Name: name, Min: Optional, Simple: e164StringType, Attrs: []Attribute{
		{Name: "x", Type: &Simple{Base: Token}},
	}}
}

// contactAddr is contact:addrType.
func contactAddr(min int) *Element {
	return &Element{NS: NSContact, Name: "addr", Min: min, Content: Seq{
		&Element{NS: NSContact, Name: "street", Min: Optional, Max: 3, Simple: optPostalLineType},
		&Element{NS: NSContact, Name: "city", Simple: postalLineType},
		&Element{NS: NSContact, Name: "sp", Min: Optional, Simple: optPostalLineType},
		&Element{NS: NSContact, Name: "pc", Min: Optional, Simple: pcType},
		&Element{NS: NSContact, Name: "cc", Simple: ccType},
	}}
}

func postalInfoTypeAttr() []Attribute {
	return []Attribute{{Name: "type", Type: postalInfoEnumType, Required: true}}
}

// contactDisclose is contact:discloseType, minOccurs 0.
func contactDisclose() *Element {
	intLoc := func(name string) *Element { // contact:intLocType
		return &Element{NS: NSContact, Name: name, Min: Optional, Max: 2, Attrs: postalInfoTypeAttr()}
	}
	return &Element{NS: NSContact, Name: "disclose", Min: Optional, Attrs: []Attribute{{Name: "flag", Type: boolean, Required: true}}, Content: Seq{
		intLoc("name"),
		intLoc("org"),
		intLoc("addr"),
		// Declared without a type: xs:anyType.
		&Element{NS: NSContact, Name: "voice", Min: Optional, AnyType: true},
		&Element{NS: NSContact, Name: "fax", Min: Optional, AnyType: true},
		&Element{NS: NSContact, Name: "email", Min: Optional, AnyType: true},
	}}
}

// ContactCheck is <contact:check>, contact:mIDType.
var ContactCheck = &Element{NS: NSContact, Name: "check", Content: Seq{
	&Element{NS: NSContact, Name: "id", Max: Unbounded, Simple: clIDType},
}}

// ContactInfo is <contact:info>, contact:authIDType.
var ContactInfo = &Element{NS: NSContact, Name: "info", Content: Seq{contactID(), authInfoType(NSContact, Optional)}}

// ContactDelete is <contact:delete>, contact:sIDType.
var ContactDelete = &Element{NS: NSContact, Name: "delete", Content: Seq{contactID()}}

// ContactCreate is <contact:create>, contact:createType.
var ContactCreate = &Element{NS: NSContact, Name: "create", Content: Seq{
	contactID(),
	&Element{NS: NSContact, Name: "postalInfo", Max: 2, Attrs: postalInfoTypeAttr(), Content: Seq{ // contact:postalInfoType
		&Element{NS: NSContact, Name: "name", Simple: postalLineType},
		&Element{NS: NSContact, Name: "org", Min: Optional, Simple: optPostalLineType},
		contactAddr(0),
	}},
	contactE164("voice"),
	contactE164("fax"),
	&Element{NS: NSContact, Name: "email", Simple: minTokenType},
	authInfoType(NSContact, 0),
	contactDisclose(),
}}

// contactAddRem is contact:addRemType, minOccurs 0.
func contactAddRem(name string) *Element {
	return &Element{NS: NSContact, Name: name, Min: Optional, Content: Seq{
		&Element{NS: NSContact, Name: "status", Max: 7, Simple: &Simple{Base: NormalizedString}, Attrs: []Attribute{ // contact:statusType
			{Name: "s", Type: contactStatusValueType, Required: true},
			{Name: "lang", Type: language},
		}},
	}}
}

// ContactUpdate is <contact:update>, contact:updateType.
var ContactUpdate = &Element{NS: NSContact, Name: "update", Content: Seq{
	contactID(),
	contactAddRem("add"),
	contactAddRem("rem"),
	&Element{NS: NSContact, Name: "chg", Min: Optional, Content: Seq{ // contact:chgType
		&Element{NS: NSContact, Name: "postalInfo", Min: Optional, Max: 2, Attrs: postalInfoTypeAttr(), Content: Seq{ // contact:chgPostalInfoType
			&Element{NS: NSContact, Name: "name", Min: Optional, Simple: postalLineType},
			&Element{NS: NSContact, Name: "org", Min: Optional, Simple: optPostalLineType},
			contactAddr(Optional),
		}},
		contactE164("voice"),
		contactE164("fax"),
		&Element{NS: NSContact, Name: "email", Min: Optional, Simple: minTokenType},
		authInfoType(NSContact, Optional),
		contactDisclose(),
	}},
}}
