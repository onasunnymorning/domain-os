package tests

import (
	"encoding/json"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// The TMCH data endpoints (ADR 0024 §4.1) against the fixture feed the
// harness serves: status before and after a refresh, the permission the
// refresh needs, and that the same generation fetched again changes
// nothing while a newer one does.
var _ = Describe("TMCHController", Ordered, func() {
	type status struct {
		Configured   bool `json:"configured"`
		Certificates *struct {
			Entries   int        `json:"entries"`
			ExpiresAt *time.Time `json:"expiresAt"`
		} `json:"certificates"`
		SMDRL *struct {
			Entries     int       `json:"entries"`
			GeneratedAt time.Time `json:"generatedAt"`
		} `json:"smdrl"`
		DNL *struct {
			Entries int    `json:"entries"`
			Source  string `json:"source"`
		} `json:"dnl"`
	}
	read := func() status {
		resp := api.GET("/tmch")
		Expect(resp.Code).To(Equal(http.StatusOK))
		var s status
		Expect(json.Unmarshal(resp.Body.Bytes(), &s)).To(Succeed())
		return s
	}
	outcomes := func(r queries.TMCHRefreshResult) map[string]string {
		m := map[string]string{}
		for _, s := range r.Sets {
			m[s.Set] = s.Outcome
		}
		return m
	}

	BeforeAll(func() {
		// The suite's database outlives a run; start from nothing stored.
		for _, table := range []string{"tmch_sets", "tmch_certificates", "tmch_smd_revocations", "tmch_dnl_labels"} {
			Expect(api.DB.Exec("DELETE FROM " + table).Error).To(Succeed())
		}
	})

	It("reports a configured feed with nothing fetched yet", func() {
		s := read()
		Expect(s.Configured).To(BeTrue())
		Expect(s.Certificates).To(BeNil())
		Expect(s.SMDRL).To(BeNil())
		Expect(s.DNL).To(BeNil())
	})

	It("refuses a refresh without registry:policy:admin", func() {
		resp := api.POSTNoBody("/tmch/refresh")
		Expect(resp.Code).To(Equal(http.StatusForbidden))
		Expect(resp.Body.String()).To(ContainSubstring("registry:policy:admin"))
	})

	It("refreshes the three sets and reports them", func() {
		resp := policyReq(http.MethodPost, "/tmch/refresh", nil)
		Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
		var r queries.TMCHRefreshResult
		Expect(json.Unmarshal(resp.Body.Bytes(), &r)).To(Succeed())
		Expect(outcomes(r)).To(Equal(map[string]string{"certificates": "refreshed", "smdrl": "refreshed", "dnl": "refreshed"}))

		s := read()
		Expect(s.Certificates.Entries).To(Equal(1))
		Expect(s.Certificates.ExpiresAt).NotTo(BeNil())
		Expect(s.SMDRL.Entries).To(Equal(1))
		Expect(s.DNL.Entries).To(Equal(2))
		Expect(s.DNL.Source).To(HavePrefix(tmchFeedServer.URL))
	})

	It("stores nothing for the same generation, and a newer one when published", func() {
		resp := policyReq(http.MethodPost, "/tmch/refresh", nil)
		Expect(resp.Code).To(Equal(http.StatusOK))
		var r queries.TMCHRefreshResult
		Expect(json.Unmarshal(resp.Body.Bytes(), &r)).To(Succeed())
		Expect(outcomes(r)).To(Equal(map[string]string{"certificates": "unchanged", "smdrl": "unchanged", "dnl": "unchanged"}))
		before := read()

		advanceTMCHFeed(24 * time.Hour)
		resp = policyReq(http.MethodPost, "/tmch/refresh", nil)
		Expect(resp.Code).To(Equal(http.StatusOK))
		Expect(json.Unmarshal(resp.Body.Bytes(), &r)).To(Succeed())
		Expect(outcomes(r)).To(Equal(map[string]string{"certificates": "unchanged", "smdrl": "refreshed", "dnl": "refreshed"}))
		after := read()
		Expect(after.SMDRL.GeneratedAt).To(Equal(before.SMDRL.GeneratedAt.Add(24 * time.Hour)))
	})
})
