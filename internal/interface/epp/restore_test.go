package epp

import (
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// The RGP restore (RFC 3915 §4.2.5) through <domain:update>.

const rgpUpdateOpen = `<rgp:update xmlns:rgp="urn:ietf:params:xml:ns:rgp-1.0">`

func restoreRequestXML(name, changes string) string {
	return domainUpdateXML(name, changes, rgpUpdateOpen+`<rgp:restore op="request"/></rgp:update>`)
}

// restoreReport is an <rgp:report>'s parts, as test cases change them.
type restoreReport struct {
	delTime, resTime string
	statements       []string
	preData          string
}

func goodRestoreReport(now time.Time) restoreReport {
	return restoreReport{
		delTime: now.Add(-48 * time.Hour).Format(time.RFC3339), resTime: now.Add(-time.Hour).Format(time.RFC3339),
		statements: []string{
			"This registrar has not restored the Registered Name in order to assume the rights to use or sell the Registered Name for itself or for any third party.",
			"The information in this report is true to best of this registrar's knowledge.",
		},
		preData: `Registrant <b xmlns="urn:example">Jane</b> &amp; co.`,
	}
}

func restoreReportXML(name string, r restoreReport) string {
	var st strings.Builder
	for _, s := range r.statements {
		st.WriteString("<rgp:statement>" + s + "</rgp:statement>")
	}
	return domainUpdateXML(name, "<domain:chg/>", rgpUpdateOpen+`<rgp:restore op="report"><rgp:report>`+
		`<rgp:preData>`+r.preData+`</rgp:preData><rgp:postData>Restored as it was.</rgp:postData>`+
		`<rgp:delTime>`+r.delTime+`</rgp:delTime><rgp:resTime>`+r.resTime+`</rgp:resTime>`+
		`<rgp:resReason>Registrant error.</rgp:resReason>`+st.String()+`</rgp:report></rgp:restore></rgp:update>`)
}

// rgpStatusesOf are the <rgp:rgpStatus> values of a response's extension.
func rgpStatusesOf(resp *etree.Document) []string {
	var out []string
	for _, el := range resp.FindElements("//extension/*/rgpStatus") {
		out = append(out, el.SelectAttrValue("s", ""))
	}
	return out
}

// deleteRestorable creates name and deletes it after its add grace period
// (a delete within it purges at once), so it is in its redemption period.
func deleteRestorable(t *testing.T, c *client, reg *memstore.Registry, name string) {
	t.Helper()
	requireCode(t, c.roundTrip(createXML(name, createOpts{})), "1000")
	reg.Domain(name).RGPStatus.AddPeriodEnd = time.Now().Add(-time.Hour)
	requireCode(t, c.roundTrip(domainDeleteXML(name)), "1001")
}

func TestDomainRestore(t *testing.T) {
	creds, _, c := domainServer(t)
	reg := creds.domains.registry
	now := time.Now().UTC()
	deleteRestorable(t, c, reg, "res.test")
	exp := exDate(t, c.roundTrip(infoXML("res.test", "")))
	assert.Equal(t, []string{"redemptionPeriod"}, rgpStatusesOf(c.roundTrip(infoXML("res.test", ""))))

	t.Run("refused: a report before any request (2304)", func(t *testing.T) {
		requireCode(t, c.roundTrip(restoreReportXML("res.test", goodRestoreReport(now))), "2304")
	})
	t.Run("refused: a restore with other changes (2306)", func(t *testing.T) {
		resp := c.roundTrip(restoreRequestXML("res.test", `<domain:add><domain:status s="clientHold"/></domain:add>`))
		requireCode(t, resp, "2306")
		assert.Contains(t, msg(resp), "separate update")
	})
	t.Run("requested: pendingRestore, the EPP status still pendingDelete", func(t *testing.T) {
		resp := c.roundTrip(restoreRequestXML("res.test", "<domain:chg/>"))
		requireCode(t, resp, "1000")
		requireSchemaValid(t, []byte(mustWrite(t, resp)))
		assert.Equal(t, []string{"pendingRestore"}, rgpStatusesOf(resp), "<rgp:upData>")
		info := c.roundTrip(infoXML("res.test", ""))
		assert.Contains(t, statusesOf(info), "pendingDelete")
		assert.NotContains(t, statusesOf(info), "pendingRestore", "an RGP status, not an EPP one")
		assert.Equal(t, []string{"pendingRestore"}, rgpStatusesOf(info))
	})
	t.Run("refused: a second request (2304)", func(t *testing.T) {
		requireCode(t, c.roundTrip(restoreRequestXML("res.test", "<domain:chg/>")), "2304")
	})
	t.Run("refused: any other update while pending (2304)", func(t *testing.T) {
		requireCode(t, c.roundTrip(domainUpdateXML("res.test", `<domain:add><domain:status s="clientHold"/></domain:add>`, "")), "2304")
	})
	t.Run("refused: an incomplete report (2003)", func(t *testing.T) {
		r := goodRestoreReport(now)
		r.statements = r.statements[:1]
		requireCode(t, c.roundTrip(restoreReportXML("res.test", r)), "2003")
		r = goodRestoreReport(now)
		r.preData = "  "
		requireCode(t, c.roundTrip(restoreReportXML("res.test", r)), "2003")
		resp := c.roundTrip(domainUpdateXML("res.test", "<domain:chg/>", rgpUpdateOpen+`<rgp:restore op="report"/></rgp:update>`))
		requireCode(t, resp, "2003")
		assert.Contains(t, msg(resp), "rgp:report")
	})
	t.Run("refused: impossible times (2004)", func(t *testing.T) {
		r := goodRestoreReport(now)
		r.delTime, r.resTime = r.resTime, r.delTime
		requireCode(t, c.roundTrip(restoreReportXML("res.test", r)), "2004")
		r = goodRestoreReport(now)
		r.resTime = now.Add(time.Hour).Format(time.RFC3339)
		requireCode(t, c.roundTrip(restoreReportXML("res.test", r)), "2004")
		r = goodRestoreReport(now)
		r.delTime = "12026-01-01T00:00:00Z" // valid xs:dateTime, but no time the registry holds
		requireCode(t, c.roundTrip(restoreReportXML("res.test", r)), "2004")
	})
	t.Run("refused: a schema-invalid report (2001)", func(t *testing.T) {
		r := goodRestoreReport(now)
		r.delTime = "2026-10-01"
		requireCode(t, c.roundTrip(restoreReportXML("res.test", r)), "2001")
	})
	t.Run("reported: restored, renewed a year, out of its end of life", func(t *testing.T) {
		resp := c.roundTrip(restoreReportXML("res.test", goodRestoreReport(now)))
		requireCode(t, resp, "1000")
		requireSchemaValid(t, []byte(mustWrite(t, resp)))
		assert.Nil(t, resp.FindElement("//extension"), "no RGP status left to report")
		info := c.roundTrip(infoXML("res.test", ""))
		assert.NotContains(t, statusesOf(info), "pendingDelete")
		assert.Empty(t, rgpStatusesOf(info))
		assert.Equal(t, exp.AddDate(1, 0, 0), exDate(t, info))
		requireCode(t, c.roundTrip(restoreRequestXML("res.test", "<domain:chg/>")), "2304") // no longer deleted
	})
	t.Run("refused: a domain not deleted (2304)", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML("live.test", createOpts{})), "1000")
		requireCode(t, c.roundTrip(restoreRequestXML("live.test", "<domain:chg/>")), "2304")
	})
	t.Run("refused: another registrar's domain (2201)", func(t *testing.T) {
		requireCode(t, c.roundTrip(restoreRequestXML("registered.test", "<domain:chg/>")), "2201")
	})
	t.Run("refused: a report after the deadline (2304)", func(t *testing.T) {
		deleteRestorable(t, c, reg, "slow.test")
		requireCode(t, c.roundTrip(restoreRequestXML("slow.test", "<domain:chg/>")), "1000")
		reg.Domain("slow.test").RGPStatus.PendingRestoreEnd = time.Now().Add(-time.Minute)
		resp := c.roundTrip(restoreReportXML("slow.test", goodRestoreReport(now)))
		requireCode(t, resp, "2304")
		assert.Contains(t, msg(resp), "request a restore again")
	})
	t.Run("a phase without the report: the request restores", func(t *testing.T) {
		deleteRestorable(t, c, reg, "quick.test")
		no := false
		reg.SetPolicy("test", func(p *entities.PhasePolicy) { p.RestoreReportRequired = &no })
		t.Cleanup(func() { reg.SetPolicy("test", func(p *entities.PhasePolicy) { p.RestoreReportRequired = nil }) })
		resp := c.roundTrip(restoreRequestXML("quick.test", "<domain:chg/>"))
		requireCode(t, resp, "1000")
		assert.Nil(t, resp.FindElement("//extension"))
		assert.NotContains(t, statusesOf(c.roundTrip(infoXML("quick.test", ""))), "pendingDelete")
	})
}

func TestDomainRestoreNeedsRGPAtLogin(t *testing.T) {
	creds, ts, _ := objectServer(t)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(loginWithServices([]string{nsDomain}, nil)), "1000")
	deleteRestorable(t, c, creds.domains.registry, "norgp.test")
	requireCode(t, c.roundTrip(restoreRequestXML("norgp.test", "<domain:chg/>")), "2103")
}

func TestRestoreReportKeepsTheRegistrarsContent(t *testing.T) {
	doc := etree.NewDocument()
	now := time.Now().UTC()
	require.NoError(t, doc.ReadFromString(restoreReportXML("x.test", goodRestoreReport(now))))
	r, err := rgpRestore(doc)
	require.NoError(t, err)
	require.NotNil(t, r.Report)
	assert.Equal(t, `Registrant <b xmlns="urn:example">Jane</b> &amp; co.`, r.Report.PreData)
	assert.Len(t, r.Report.Statements, 2)
	assert.WithinDuration(t, now.Add(-48*time.Hour), r.Report.DelTime, time.Second)
	assert.Equal(t, time.UTC, r.Report.DelTime.Location())
}
