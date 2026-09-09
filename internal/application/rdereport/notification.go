package rdereport

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Notification is the <rdeNotification:notification> element (draft-26 §9.3).
type Notification struct {
	XMLName           xml.Name `xml:"rdeNotification:notification"`
	XMLNSNotification string   `xml:"xmlns:rdeNotification,attr"`
	XMLNSReport       string   `xml:"xmlns:rdeReport,attr"`
	XMLNSHeader       string   `xml:"xmlns:rdeHeader,attr"`
	XMLNSIIRDEA       string   `xml:"xmlns:iirdea,attr,omitempty"`

	DEAName      string   `xml:"rdeNotification:deaName"`
	Version      int      `xml:"rdeNotification:version"`
	RepDate      string   `xml:"rdeNotification:repDate"`
	Status       string   `xml:"rdeNotification:status"`
	Results      *Results `xml:"rdeNotification:results,omitempty"`
	ReDate       string   `xml:"rdeNotification:reDate,omitempty"`
	VaDate       string   `xml:"rdeNotification:vaDate,omitempty"`
	LastFullDate string   `xml:"rdeNotification:lastFullDate,omitempty"`
	Report       *Report  `xml:"rdeReport:report,omitempty"`
}

// Results wraps the <iirdea:result> entries of a DVFN.
type Results struct {
	Result []Result `xml:"iirdea:result"`
}

// Result is one <iirdea:result> (draft-26 §9.1).
type Result struct {
	Code        int    `xml:"code,attr"`
	Msg         string `xml:"iirdea:msg"`
	Description string `xml:"iirdea:description,omitempty"`
}

// BuildNotification assembles the DVPN or DVFN for a decided run. An ERROR
// outcome yields ErrNoNotificationForOutcome: the service could not decide
// and claims nothing.
func BuildNotification(p Params) (*Notification, error) {
	status := rdevalidate.NotificationStatus(p.Result.Outcome)
	if status == entities.EscrowNotificationNone {
		return nil, fmt.Errorf("%w: outcome %q", ErrNoNotificationForOutcome, p.Result.Outcome)
	}
	if strings.TrimSpace(p.DEAName) == "" {
		return nil, errors.New("rdereport: DEAName is required")
	}
	if len(p.DEAName) > 255 {
		return nil, errors.New("rdereport: DEAName exceeds 255 characters")
	}
	if p.ValidatedAt.IsZero() {
		return nil, errors.New("rdereport: ValidatedAt is required")
	}
	report, err := BuildReport(p)
	if err != nil {
		return nil, err
	}
	repDate, err := report.WatermarkDate()
	if err != nil {
		return nil, fmt.Errorf("rdereport: watermark: %w", err)
	}

	n := &Notification{
		XMLNSNotification: NSNotification,
		XMLNSReport:       NSReport,
		XMLNSHeader:       NSHeader,
		DEAName:           strings.TrimSpace(p.DEAName),
		Version:           notificationVersion,
		RepDate:           repDate,
		Status:            string(status),
		ReDate:            fmtDateTime(p.ReceivedAt),
		VaDate:            fmtDateTime(p.ValidatedAt),
		Report:            report,
	}
	if p.LastFullDate != nil {
		n.LastFullDate = fmtDate(*p.LastFullDate)
	}
	if status == entities.EscrowNotificationDVFN {
		n.XMLNSIIRDEA = NSIIRDEA
		n.Results = resultsFromFindings(p.Result.Findings)
	}
	return n, nil
}

// resultsFromFindings converts ERROR-severity findings into iirdea results.
// The message is the code's canonical text; the finding's own (template-only)
// message and locator go into the description.
//
// Finding.Object is deliberately not among them. A DVFN leaves this system for
// ICANN, and everything it carries originates as bytes from an untrusted
// deposit; the description is therefore built from constant templates and
// numbers only, exactly as it was before findings learned to name an object.
// The operator gets the names in the summary, which stays in the tenant's
// bucket. TestResultsFromFindings_OmitsTheObject holds this.
func resultsFromFindings(fs []rdevalidate.Finding) *Results {
	var out []Result
	for _, f := range fs {
		if f.Severity != rdevalidate.SeverityError {
			continue
		}
		code, msg := ResultCodeFor(f.Code)
		desc := f.Message
		if f.Locator != "" {
			desc += " (" + f.Locator + ")"
		}
		out = append(out, Result{Code: code, Msg: msg, Description: desc})
	}
	if len(out) == 0 {
		// A DVFN always carries at least one result; the schema requires it.
		code, msg := ResultCodeFor("")
		out = append(out, Result{Code: code, Msg: msg})
	}
	return &Results{Result: out}
}

// Marshal renders the notification document.
func (n *Notification) Marshal() ([]byte, error) {
	return marshalDoc(n)
}
