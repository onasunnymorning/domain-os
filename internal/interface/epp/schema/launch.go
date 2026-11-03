package schema

// Models of launch-1.0.xsd (RFC 8334), for the launch phase extension a
// client puts in <extension>: <launch:check>, <launch:create>,
// <launch:info>, <launch:update> and <launch:delete>. Names in comments are
// the XSD type each model mirrors.
//
// Of the trademark evidence a <launch:create> may carry (RFC 8334 §2.6),
// only <smd:encodedSignedMark>, the form ICANN's SMD files take, and a
// <launch:codeMark> with a <launch:code> are modelled. A decoded
// <smd:signedMark> and an inline <mark:mark> are not: they are the mark
// and XML Signature schemas in full, which sunrise support (roadmap 9b)
// models when it verifies them; until then a frame carrying one is 2001,
// and a frame carrying any evidence is refused by the handler (2306), since
// none of it is verified yet. testdata/rst/PROVENANCE.md records the same.

// Namespaces of launch-1.0 and the schemas it imports.
const (
	NSLaunch     = "urn:ietf:params:xml:ns:launch-1.0"
	NSMark       = "urn:ietf:params:xml:ns:mark-1.0"
	NSSignedMark = "urn:ietf:params:xml:ns:signedMark-1.0"
)

var (
	// launch:phaseTypeValue
	launchPhaseValue = &Simple{Base: Token, Enum: []string{"sunrise", "landrush", "claims", "open", "custom"}}
	// launch:objectType
	launchObjectType = &Simple{Base: Token, Enum: []string{"application", "registration"}}
	// launch:checkFormType
	launchCheckForm = &Simple{Base: Token, Enum: []string{"claims", "avail", "trademark"}}
	// xs:token, as launch:applicationIDType and the name attribute of
	// launch:phaseType.
	launchToken = &Simple{Base: Token}
	// launch:codeValue, launch:noticeIDValue and launch:validatorIDType: a
	// token of at least one character.
	launchMinToken = &Simple{Base: Token, MinLen: 1}
)

// launchPhase is launch:phaseType: one of the five phase values, with an
// optional name that extends it (a custom phase's name).
func launchPhase(min int) *Element {
	return &Element{NS: NSLaunch, Name: "phase", Min: min, Simple: launchPhaseValue, Attrs: []Attribute{
		{Name: "name", Type: launchToken},
	}}
}

// launchApplicationID is launch:applicationIDType.
func launchApplicationID(min int) *Element {
	return &Element{NS: NSLaunch, Name: "applicationID", Min: min, Simple: launchToken}
}

// launchIDContainer is launch:idContainerType: a phase and an application
// identifier.
func launchIDContainer(name string) *Element {
	return &Element{NS: NSLaunch, Name: name, Content: Seq{launchPhase(0), launchApplicationID(0)}}
}

// LaunchCheck is <launch:check>, launch:checkType: the claims check form
// (the default), the availability check form or the trademark check form,
// in a phase.
var LaunchCheck = &Element{NS: NSLaunch, Name: "check", Attrs: []Attribute{{Name: "type", Type: launchCheckForm}}, Content: Seq{
	launchPhase(Optional),
}}

// LaunchCreate is <launch:create>, launch:createType: the phase, the
// trademark evidence (see the package comment for what is modelled) and
// the claims notices.
var LaunchCreate = &Element{NS: NSLaunch, Name: "create", Attrs: []Attribute{{Name: "type", Type: launchObjectType}}, Content: Seq{
	launchPhase(0),
	Choice{Min: Optional, Alts: []Particle{
		&Element{NS: NSLaunch, Name: "codeMark", Max: Unbounded, Content: Seq{ // launch:codeMarkType, without mark:abstractMark
			&Element{NS: NSLaunch, Name: "code", Min: Optional, Simple: launchMinToken, Attrs: []Attribute{ // launch:codeType
				{Name: "validatorID", Type: launchMinToken},
			}},
		}},
		&Element{NS: NSSignedMark, Name: "encodedSignedMark", Max: Unbounded, Simple: launchToken, Attrs: []Attribute{ // smd:encodedSignedMarkType
			{Name: "encoding", Type: launchToken},
		}},
	}},
	&Element{NS: NSLaunch, Name: "notice", Min: Optional, Max: Unbounded, Content: Seq{ // launch:createNoticeType
		&Element{NS: NSLaunch, Name: "noticeID", Simple: launchMinToken, Attrs: []Attribute{ // launch:noticeIDType
			{Name: "validatorID", Type: launchMinToken},
		}},
		&Element{NS: NSLaunch, Name: "notAfter", Simple: dateTime},
		&Element{NS: NSLaunch, Name: "acceptedDate", Simple: dateTime},
	}},
}}

// LaunchInfo is <launch:info>, launch:infoType.
var LaunchInfo = &Element{NS: NSLaunch, Name: "info", Attrs: []Attribute{{Name: "includeMark", Type: boolean}}, Content: Seq{
	launchPhase(0),
	launchApplicationID(Optional),
}}

// LaunchUpdate and LaunchDelete are <launch:update> and <launch:delete>,
// both launch:idContainerType.
var (
	LaunchUpdate = launchIDContainer("update")
	LaunchDelete = launchIDContainer("delete")
)
