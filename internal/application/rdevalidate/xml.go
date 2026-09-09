package rdevalidate

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// heartbeatEvery is the object interval at which the validator reports progress.
const heartbeatEvery = 1000

// XMLValidator streams an RDE deposit (RFC 8909 wrapper, RFC 9022 objects)
// and turns every structural or content problem into a Finding. It reuses
// the pkg/domain/entities RDE structs and namespace constants that the import
// parser uses, but writes nothing, logs nothing, and never drops an element
// silently.
type XMLValidator struct {
	BoundTLD  string // TLD from the authenticated intake context; the header must agree
	Now       func() time.Time
	Heartbeat func(stage Stage, detail string)
}

// objectSpec is the per-object decode/validate table.
type objectSpec struct {
	uri    string
	name   string
	decode func(dec *xml.Decoder, se *xml.StartElement) objectResult
}

// objectResult is what a per-object decoder reports back about one object.
// At most one of the three problems is set.
type objectResult struct {
	// Missing lists the RDE-required elements the object does not carry.
	// ERROR severity: the deposit does not conform to RFC 9022.
	Missing error
	// Rejected is the error the domain-os entity constructor returned for an
	// object that carries everything RDE requires. WARNING severity: the
	// deposit is well-formed, this registry would not import it as it stands.
	Rejected error
	// Rule names the rule behind Rejected in constant, value-free terms. See
	// entityRule — the error itself can carry payload text and is never used.
	Rule string
	// DecodeErr is a failure to decode the element at all.
	DecodeErr error

	// Declares is the identifier this object adds to the deposit's namespace:
	// a contact id or a host name. Refs are the contact ids and RefHosts the
	// nameservers it points at. They feed crossRef and nothing else — they are
	// payload values and must never reach a finding.
	Declares string
	Refs     []string
	RefHosts []string
}

var objectSpecs = []objectSpec{
	{uri: entities.DOMAIN_URI, name: "domain", decode: decodeDomain},
	{uri: entities.CONTACT_URI, name: "contact", decode: decodeContact},
	{uri: entities.HOST_URI, name: "host", decode: decodeHost},
	{uri: entities.REGISTRAR_URI, name: "registrar", decode: decodeRegistrar},
	{uri: entities.IDN_URI, name: "idnTableRef", decode: decodeIDN},
	{uri: entities.NNDN_URI, name: "NNDN", decode: decodeNNDN},
}

// CountsObjectsIn reports whether this validator walks and counts the objects
// of a namespace. A deposit's header may declare namespaces that carry no
// countable objects — rdeEppParams and rdePolicy each declare a count of one
// for a single non-repeating element — and a report must not present those as
// a count mismatch.
func CountsObjectsIn(namespaceURI string) bool {
	for _, s := range objectSpecs {
		if s.uri == namespaceURI {
			return true
		}
	}
	return false
}

// Validate streams r to EOF. It always returns a summary (possibly partial)
// and the findings it produced; the caller decides the outcome.
func (v *XMLValidator) Validate(ctx context.Context, r io.Reader) (DepositSummary, []Finding) {
	now := v.Now
	if now == nil {
		now = time.Now
	}
	summary := DepositSummary{Observed: map[string]int{}}
	var findings []Finding
	addRuled := func(code Code, sev Severity, stage Stage, objType, locator, rule, msg string) {
		findings = append(findings, Finding{
			Code: code, Severity: sev, Stage: stage, Rule: rule,
			ObjectType: objType, Locator: locator, Message: msg, At: now(),
		})
	}
	add := func(code Code, sev Severity, stage Stage, objType, locator, msg string) {
		addRuled(code, sev, stage, objType, locator, "", msg)
	}
	specByLocal := map[string]objectSpec{}
	for _, s := range objectSpecs {
		specByLocal[s.name] = s
	}
	xref := newCrossRef(v.BoundTLD)

	dec := xml.NewDecoder(r)
	dec.Strict = true
	depositFound, headerFound := false, false
	objects := 0

	for {
		if objects%256 == 0 {
			if err := ctx.Err(); err != nil {
				add(CodeValidationTimeout, SeverityError, StageXML, "", offsetLocator(dec), "validation deadline reached while streaming the deposit")
				return summary, findings
			}
		}
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			var syn *xml.SyntaxError
			switch {
			case errors.Is(err, errBudgetExceeded):
				add(CodeArchiveLimitUnpackedSize, SeverityError, StageUnpack, "archive", offsetLocator(dec), "unpacked size limit exceeded while streaming the deposit")
			case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
				add(CodeValidationTimeout, SeverityError, StageXML, "", offsetLocator(dec), "validation deadline reached while streaming the deposit")
			case errors.As(err, &syn):
				add(CodeXMLMalformed, SeverityError, StageXML, "", "line="+itoa(syn.Line), "deposit XML is not well-formed")
			case IsIntegrityError(err):
				add(CodeDecryptFailed, SeverityError, StageDecrypt, "deposit", "", "deposit integrity check failed while streaming: ciphertext was altered")
			default:
				add(CodeXMLMalformed, SeverityError, StageXML, "", offsetLocator(dec), "deposit XML could not be read")
			}
			return summary, findings
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		local, space := se.Name.Local, se.Name.Space

		switch {
		case local == "deposit" && space == entities.RDE_URI:
			if depositFound {
				add(CodeRDEObjectInvalid, SeverityError, StageRDE, "deposit", offsetLocator(dec), "more than one deposit element")
				continue
			}
			depositFound = true
			v.parseDepositAttrs(se, &summary, add, dec)
			// Only a FULL deposit is self-contained. In a DIFF or INCR, a
			// contact whose domain was deposited last week is not an orphan
			// and a reference into an earlier deposit is not dangling.
			xref.enabled = summary.Kind == entities.RDEReportTypeFULL
		case local == "watermark" && space == entities.RDE_URI:
			var wm string
			if err := dec.DecodeElement(&wm, &se); err != nil {
				add(CodeRDEObjectDecodeError, SeverityError, StageRDE, "deposit", offsetLocator(dec), "watermark element could not be decoded")
				continue
			}
			t, err := time.Parse(time.RFC3339, strings.TrimSpace(wm))
			if err != nil {
				add(CodeRDEObjectInvalid, SeverityError, StageRDE, "deposit", offsetLocator(dec), "watermark is not an RFC 3339 timestamp")
				continue
			}
			summary.Watermark = t.UTC()
		case local == "header" && space == entities.RDE_HEADER_URI:
			if headerFound {
				add(CodeRDEObjectInvalid, SeverityError, StageRDE, "header", offsetLocator(dec), "more than one header element")
				continue
			}
			var h entities.RDEHeader
			if err := dec.DecodeElement(&h, &se); err != nil {
				add(CodeRDEObjectDecodeError, SeverityError, StageRDE, "header", offsetLocator(dec), "header element could not be decoded")
				continue
			}
			headerFound = true
			summary.Header = h
			summary.HeaderFound = true
			v.checkHeader(h, add, dec)
		default:
			spec, known := specByLocal[local]
			if !known || spec.uri != space {
				continue
			}
			objects++
			if objects%heartbeatEvery == 0 && v.Heartbeat != nil {
				v.Heartbeat(StageRDE, "objects="+itoa(objects))
			}
			out := spec.decode(dec, &se)
			locator := spec.name + "#" + itoa(summary.Observed[spec.uri]+1) + " " + offsetLocator(dec)
			var syn *xml.SyntaxError
			switch {
			case out.DecodeErr != nil && errors.As(out.DecodeErr, &syn):
				// The document itself is broken inside this object: report the
				// well-formedness failure once and stop, like the token loop does.
				add(CodeXMLMalformed, SeverityError, StageXML, "", "line="+itoa(syn.Line), "deposit XML is not well-formed")
				return summary, findings
			case out.DecodeErr != nil:
				add(CodeRDEObjectDecodeError, SeverityError, StageRDE, spec.name, locator, "object could not be decoded")
			case out.Missing != nil:
				// The element names come from requireAll's own constant list,
				// never from the deposit, so they are safe as a rule.
				addRuled(CodeRDEObjectInvalid, SeverityError, StageRDE, spec.name, locator,
					"missing RDE-required element(s): "+out.Missing.Error(),
					"object is missing an RDE-required element: "+out.Missing.Error())
			case out.Rejected != nil:
				addRuled(CodeRDEObjectEntityRejected, SeverityWarning, StageRDE, spec.name, locator, out.Rule,
					"object is well-formed but this registry would not import it as it stands: "+out.Rule)
			}
			ordinal := summary.Observed[spec.uri] + 1
			xref.declare(spec.name, out, ordinal)
			summary.Observed[spec.uri]++
		}
	}

	if !depositFound {
		add(CodeXMLNoDeposit, SeverityError, StageXML, "deposit", "", "no rde:deposit element found")
	}
	if !headerFound {
		add(CodeXMLNoHeader, SeverityError, StageXML, "header", "", "no rdeHeader:header element found")
		return summary, findings
	}
	if summary.Watermark.IsZero() {
		add(CodeRDEObjectInvalid, SeverityError, StageRDE, "deposit", "", "deposit has no watermark")
	}

	// Header counts vs observed counts, per object namespace.
	declared := map[string]int{}
	for _, c := range summary.Header.Count {
		declared[c.Uri] = c.ID
		if c.ID < 0 {
			add(CodeRDEObjectInvalid, SeverityError, StageRDE, "header", "", "header declares a negative count")
		}
	}
	for _, s := range objectSpecs {
		exp, declaredHere := declared[s.uri]
		got := summary.Observed[s.uri]
		switch {
		case declaredHere && exp > 0 && got == 0:
			add(CodeRDERequiredObjectMissing, SeverityError, StageRDE, s.name, "", "header declares "+itoa(exp)+" objects but none were found")
		case declaredHere && exp != got:
			add(CodeRDECountMismatch, SeverityError, StageRDE, s.name, "", "header declares "+itoa(exp)+" objects, deposit contains "+itoa(got))
		case !declaredHere && got > 0:
			add(CodeRDECountMismatch, SeverityError, StageRDE, s.name, "", "deposit contains "+itoa(got)+" objects the header does not declare")
		}
	}

	// Referential integrity across the whole deposit. It runs last because it
	// is the only check that needs to have seen every object.
	xref.report(addRuled)
	return summary, findings
}

func (v *XMLValidator) parseDepositAttrs(se xml.StartElement, s *DepositSummary, add func(Code, Severity, Stage, string, string, string), dec *xml.Decoder) {
	for _, a := range se.Attr {
		switch a.Name.Local {
		case "type":
			s.Kind = strings.ToUpper(strings.TrimSpace(a.Value))
		case "id":
			s.ID = strings.TrimSpace(a.Value)
		case "prevId":
			s.PrevID = strings.TrimSpace(a.Value)
		case "resend":
			n, err := strconv.Atoi(strings.TrimSpace(a.Value))
			if err != nil || n < 0 {
				add(CodeRDEObjectInvalid, SeverityError, StageRDE, "deposit", offsetLocator(dec), "deposit resend attribute is not a non-negative integer")
				continue
			}
			s.Resend = n
		}
	}
	switch s.Kind {
	case entities.RDEReportTypeFULL, entities.RDEReportTypeDIFF, entities.RDEReportTypeINCR:
	default:
		add(CodeRDEObjectInvalid, SeverityError, StageRDE, "deposit", offsetLocator(dec), "deposit type attribute must be FULL, DIFF or INCR")
	}
	if s.ID == "" {
		add(CodeRDEObjectInvalid, SeverityError, StageRDE, "deposit", offsetLocator(dec), "deposit id attribute is missing")
	}
	if s.Kind != entities.RDEReportTypeFULL && s.PrevID == "" {
		add(CodeRDEObjectInvalid, SeverityError, StageRDE, "deposit", offsetLocator(dec), "non-FULL deposit must carry a prevId attribute")
	}
}

func (v *XMLValidator) checkHeader(h entities.RDEHeader, add func(Code, Severity, Stage, string, string, string), dec *xml.Decoder) {
	tld := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h.TLD)), ".")
	if tld == "" {
		add(CodeRDEObjectInvalid, SeverityError, StageRDE, "header", offsetLocator(dec), "header has no tld element")
		return
	}
	if v.BoundTLD != "" && tld != v.BoundTLD {
		add(CodeRDEHeaderTLDMismatch, SeverityError, StageRDE, "header", offsetLocator(dec), "header tld does not match the TLD the deposit was bound to at intake")
	}
}

func offsetLocator(dec *xml.Decoder) string {
	return "offset=" + i64toa(dec.InputOffset())
}

// --- per-object decoders -------------------------------------------------

func requireAll(pairs ...string) error {
	var missing []string
	for i := 0; i+1 < len(pairs); i += 2 {
		if strings.TrimSpace(pairs[i+1]) == "" {
			missing = append(missing, pairs[i])
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return errors.New(strings.Join(missing, ","))
}

func decodeDomain(dec *xml.Decoder, se *xml.StartElement) objectResult {
	var d entities.RDEDomain
	if err := dec.DecodeElement(&d, se); err != nil {
		return objectResult{DecodeErr: err}
	}
	// RFC 9022 §4.1: name, roid, status and clID are the required elements.
	// crRr, crDate, exDate, upRr and upDate are all minOccurs="0" — a deposit
	// that leaves one out is conformant, not defective.
	// What the domain points at. Collected even when the object is rejected
	// below: a domain with a bad roid still uses its contacts and nameservers,
	// and calling them orphans because of it would be wrong.
	res := objectResult{Refs: domainContacts(&d), RefHosts: domainHosts(&d)}

	req := requireAll("name", string(d.Name), "roid", d.RoID, "clID", d.ClID)
	if req == nil && len(d.Status) == 0 {
		req = errors.New("status")
	}
	if req != nil {
		res.Missing = req
		return res
	}
	_, err := d.ToEntity()
	res.Rejected = err
	res.Rule = entityRule(err, "exDate", d.ExDate, "crDate", d.CrDate, "upDate", d.UpDate)
	return res
}

func decodeContact(dec *xml.Decoder, se *xml.StartElement) objectResult {
	var c entities.RDEContact
	if err := dec.DecodeElement(&c, se); err != nil {
		return objectResult{DecodeErr: err}
	}
	// RFC 9022 §5.1: id, roid, status, postalInfo, email and clID are required.
	req := requireAll("id", c.ID, "roid", c.RoID, "email", c.Email, "clID", c.ClID)
	if req == nil && len(c.Status) == 0 {
		req = errors.New("status")
	}
	if req == nil && len(c.PostalInfo) == 0 {
		req = errors.New("postalInfo")
	}
	res := objectResult{Declares: c.ID}
	if req != nil {
		res.Missing = req
		return res
	}
	_, err := c.ToEntity()
	res.Rejected = err
	res.Rule = entityRule(err, "crDate", c.CrDate, "upDate", c.UpDate)
	return res
}

func decodeHost(dec *xml.Decoder, se *xml.StartElement) objectResult {
	var h entities.RDEHost
	if err := dec.DecodeElement(&h, se); err != nil {
		return objectResult{DecodeErr: err}
	}
	// RFC 9022 §6.1: name, roid, status and clID are required; addr is not, an
	// out-of-bailiwick host having none.
	req := requireAll("name", h.Name, "roid", h.RoID, "clID", h.ClID)
	if req == nil && len(h.Status) == 0 {
		req = errors.New("status")
	}
	res := objectResult{Declares: h.Name}
	if req != nil {
		res.Missing = req
		return res
	}
	_, err := h.ToEntity()
	res.Rejected = err
	res.Rule = entityRule(err, "crDate", h.CrDate, "upDate", h.UpDate)
	return res
}

func decodeRegistrar(dec *xml.Decoder, se *xml.StartElement) objectResult {
	var r entities.RDERegistrar
	if err := dec.DecodeElement(&r, se); err != nil {
		return objectResult{DecodeErr: err}
	}
	// RFC 9022 §7.1: only id and name are required. gurid, status, postalInfo,
	// voice, fax, email, url, whoisInfo, crDate and upDate are all
	// minOccurs="0" — a registrar with no postal address is conformant.
	req := requireAll("id", r.ID, "name", r.Name)
	if req != nil {
		return objectResult{Missing: req}
	}
	_, err := r.ToEntity()
	return objectResult{Rejected: err, Rule: entityRule(err, "crDate", r.CrDate, "upDate", r.UpDate)}
}

func decodeIDN(dec *xml.Decoder, se *xml.StartElement) objectResult {
	var i entities.RDEIdnTableReference
	if err := dec.DecodeElement(&i, se); err != nil {
		return objectResult{DecodeErr: err}
	}
	// RFC 9022 §8.1: url and urlPolicy are required elements and id is a
	// required attribute of idnTableRef.
	return objectResult{Missing: requireAll("id", i.ID, "url", i.Url, "urlPolicy", i.UrlPolicy)}
}

func decodeNNDN(dec *xml.Decoder, se *xml.StartElement) objectResult {
	var n entities.RDENNDN
	if err := dec.DecodeElement(&n, se); err != nil {
		return objectResult{DecodeErr: err}
	}
	// RFC 9022 §9.1: aName and nameState are required; crDate is not.
	return objectResult{Missing: requireAll("aName", n.AName, "nameState", n.NameState)}
}

// domainContacts is every contact id the domain points at: the registrant and
// each linked contact, whatever its type.
func domainContacts(d *entities.RDEDomain) []string {
	ids := make([]string, 0, len(d.Contact)+1)
	if d.Registrant != "" {
		ids = append(ids, d.Registrant)
	}
	for _, c := range d.Contact {
		if c.ID != "" {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

// domainHosts is every nameserver the domain delegates to by name.
//
// RFC 9022 also allows a domain to carry its nameservers inline as hostAttr
// rather than by reference. entities.RDEDomain does not model that form, so a
// deposit using it contributes no host references here and its hosts are not
// cross-checked — the check reports nothing rather than reporting every host
// as an orphan.
func domainHosts(d *entities.RDEDomain) []string {
	var names []string
	for _, ns := range d.Ns {
		for _, h := range ns.HostObjs {
			if h != "" {
				names = append(names, h)
			}
		}
	}
	return names
}
