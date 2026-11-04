package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Launch applications admin API (ADR 0024 §3)
//
// Story: registrars filed applications for a name in an application phase
// (over EPP; here written through the repository, as the EPP server would);
// the operator lists them, validates one, allocates the name to it, and the
// other is rejected, each applicant told by poll message.

// launchReq performs a request as the TLD's operator (X-Tenant-ID) with the
// registry:launch:admin permission when permitted is set.
func launchReq(method, path, tenant string, permitted bool, body interface{}) *httptest.ResponseRecorder {
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	req, err := http.NewRequest(method, api.Server.URL+path, bytes.NewReader(payload))
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenant)
	if permitted {
		req.Header.Set(TestAuthScopesHeader, rest.ScopeRegistryLaunchAdmin)
	}
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	return toRecorder(resp)
}

var _ = Describe("LaunchApplicationController", Ordered, func() {
	const (
		ryID      = "lnchAppRyOp"
		otherRyID = "lnchAppOther"
		tldName   = "lnchapp"
		phaseName = "auction"
		rar1      = "lnchRar1"
		rar2      = "lnchRar2"
	)
	var (
		repo          *postgres.LaunchApplicationRepository
		winner, loser *entities.LaunchApplication
		unrelated     *entities.LaunchApplication
	)

	BeforeAll(func() {
		for _, ry := range []string{ryID, otherRyID} {
			resp := api.POST("/registry-operators", &commands.CreateRegistryOperatorCommand{RyID: ry, Name: ry, Email: ry + "@example.com"})
			Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
		}
		resp := api.POST("/tlds", &request.CreateTLDRequest{Name: tldName, RyID: ryID})
		Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
		resp = api.POST(fmt.Sprintf("/tlds/%s/phases", tldName), &commands.CreatePhaseCommand{
			Name: phaseName, Type: "Launch", Starts: time.Now().UTC().Add(-48 * time.Hour),
			Launch: &entities.LaunchSettings{Mode: entities.LaunchModeLandrush, Model: entities.RegistrationModelApplication, AcceptSignedMarks: true},
		})
		Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
		resp = api.POST(fmt.Sprintf("/tlds/%s/phases/%s/prices", tldName, phaseName), &commands.CreatePriceCommand{Currency: "USD", RegistrationAmount: 1000, RenewalAmount: 1000, TransferAmount: 1000, RestoreAmount: 1000})
		Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
		for i, rar := range []string{rar1, rar2} {
			resp = api.POST("/registrars", testRegistrar(rar, rar, 9100+i))
			Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
			resp = api.PUT(fmt.Sprintf("/registrars/%s/status/ok", rar), nil)
			Expect(resp.Code).To(SatisfyAny(Equal(http.StatusOK), Equal(http.StatusNoContent)))
			resp = api.PUT(fmt.Sprintf("/registrars/%s/iana_status/Accredited", rar), nil)
			Expect(resp.Code).To(SatisfyAny(Equal(http.StatusOK), Equal(http.StatusNoContent)))
			resp = api.POSTNoBody(fmt.Sprintf("/accreditations/%s/%s", tldName, rar))
			Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
			addr, err := entities.NewAddress("Brussels", "BE")
			Expect(err).NotTo(HaveOccurred())
			postal, err := entities.NewContactPostalInfo("int", "Test Contact", addr)
			Expect(err).NotTo(HaveOccurred())
			resp = api.POST("/contacts", &commands.CreateContactCommand{
				ID: "c-" + rar, Email: "c@" + rar + ".example.com", AuthInfo: "Tr4nsf3r-C0de-Xq9#Lm2!", ClID: rar,
				PostalInfo: [2]*entities.ContactPostalInfo{postal, nil},
			})
			Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
		}

		// The applications, as the EPP server would have filed them.
		repo = postgres.NewLaunchApplicationRepository(api.DB, "APEX")
		phase, err := postgres.NewGormPhaseRepository(api.DB).GetPhaseByTLDAndName(context.Background(), tldName, phaseName)
		Expect(err).NotTo(HaveOccurred())
		file := func(id int64, name, rar string) *entities.LaunchApplication {
			roid, err := entities.NewRoidType(id, entities.RoidTypeApplication, "APEX")
			Expect(err).NotTo(HaveOccurred())
			a, err := entities.NewLaunchApplication(roid, name, rar, "Tr4nsf3r-C0de-Xq9#Lm2!", phase, 1, time.Now().UTC())
			Expect(err).NotTo(HaveOccurred())
			a.RegistrantID, a.TechID = entities.ClIDType("c-"+rar), entities.ClIDType("c-"+rar)
			a.Quote = entities.ApplicationQuote{Currency: "USD", Amount: 1000, Class: "standard"}
			stored, err := repo.CreateApplication(context.Background(), a)
			Expect(err).NotTo(HaveOccurred())
			return stored
		}
		winner = file(910001, "prize."+tldName, rar1)
		loser = file(910002, "prize."+tldName, rar2)
		unrelated = file(910003, "spare."+tldName, rar1)
	})

	AfterAll(func() {
		api.DB.Where("tld_name = ?", tldName).Delete(&postgres.LaunchApplicationRecord{})
		api.DB.Where("cl_id IN ?", []string{rar1, rar2}).Delete(&postgres.PollMessageRecord{})
	})

	It("lists the applications filed in the phase, by status", func() {
		resp := launchReq(http.MethodGet, fmt.Sprintf("/tlds/%s/phases/%s/applications", tldName, phaseName), ryID, false, nil)
		Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
		var apps []entities.LaunchApplication
		Expect(DecodeJSON(resp, &apps)).To(Succeed())
		Expect(apps).To(HaveLen(3))
		resp = launchReq(http.MethodGet, fmt.Sprintf("/tlds/%s/phases/%s/applications?status=allocated", tldName, phaseName), ryID, false, nil)
		Expect(resp.Code).To(Equal(http.StatusOK))
		Expect(DecodeJSON(resp, &apps)).To(Succeed())
		Expect(apps).To(BeEmpty())
		resp = launchReq(http.MethodGet, fmt.Sprintf("/tlds/%s/phases/%s/applications?status=nope", tldName, phaseName), ryID, false, nil)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("confines reads to the TLD's operator", func() {
		resp := launchReq(http.MethodGet, fmt.Sprintf("/tlds/%s/phases/%s/applications", tldName, phaseName), otherRyID, false, nil)
		Expect(resp.Code).To(Equal(http.StatusNotFound))
		resp = launchReq(http.MethodGet, "/launch-applications/"+winner.ID.String(), otherRyID, false, nil)
		Expect(resp.Code).To(Equal(http.StatusNotFound))
		resp = launchReq(http.MethodGet, "/launch-applications/"+winner.ID.String(), ryID, false, nil)
		Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
		var app entities.LaunchApplication
		Expect(DecodeJSON(resp, &app)).To(Succeed())
		Expect(app.ID).To(Equal(winner.ID))
		Expect(app.Status).To(Equal(entities.ApplicationStatusPendingAllocation))
		resp = launchReq(http.MethodGet, "/launch-applications/9_APP-APEX", ryID, false, nil)
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	It("needs registry:launch:admin to decide", func() {
		resp := launchReq(http.MethodPost, "/launch-applications/"+winner.ID.String()+"/allocate", ryID, false, nil)
		Expect(resp.Code).To(Equal(http.StatusForbidden))
		resp = launchReq(http.MethodPost, "/launch-applications/"+winner.ID.String()+"/reject", ryID, false, &commands.RejectApplicationCommand{Reason: "x"})
		Expect(resp.Code).To(Equal(http.StatusForbidden))
	})

	It("refuses to validate an application that is not pendingValidation", func() {
		resp := launchReq(http.MethodPost, "/launch-applications/"+winner.ID.String()+"/validate", ryID, true, &commands.ValidateApplicationCommand{Valid: true})
		Expect(resp.Code).To(Equal(http.StatusConflict), resp.Body.String())
	})

	It("allocates the name to the winner and rejects the other application", func() {
		resp := launchReq(http.MethodPost, "/launch-applications/"+winner.ID.String()+"/allocate", ryID, true, nil)
		Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
		var app entities.LaunchApplication
		Expect(DecodeJSON(resp, &app)).To(Succeed())
		Expect(app.Status).To(Equal(entities.ApplicationStatusAllocated))
		Expect(app.DomainRoID).NotTo(BeEmpty())

		resp = api.GET("/domains/prize." + tldName)
		Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
		var dom entities.Domain
		Expect(DecodeJSON(resp, &dom)).To(Succeed())
		Expect(dom.ClID).To(Equal(entities.ClIDType(rar1)))
		Expect(dom.RegistrationPhaseName).To(Equal(entities.ClIDType(phaseName)))

		resp = launchReq(http.MethodGet, "/launch-applications/"+loser.ID.String(), ryID, false, nil)
		Expect(resp.Code).To(Equal(http.StatusOK))
		Expect(DecodeJSON(resp, &app)).To(Succeed())
		Expect(app.Status).To(Equal(entities.ApplicationStatusRejected))
		Expect(app.Reason).To(ContainSubstring("allocated to another application"))

		var msgs int64
		Expect(api.DB.Model(&postgres.PollMessageRecord{}).Where("cl_id IN ?", []string{rar1, rar2}).Count(&msgs).Error).To(Succeed())
		Expect(msgs).To(Equal(int64(2)), "each applicant is told")

		resp = launchReq(http.MethodPost, "/launch-applications/"+winner.ID.String()+"/allocate", ryID, true, nil)
		Expect(resp.Code).To(Equal(http.StatusConflict), "allocated already")
	})

	It("rejects an application with a reason", func() {
		resp := launchReq(http.MethodPost, "/launch-applications/"+unrelated.ID.String()+"/reject", ryID, true, &commands.RejectApplicationCommand{})
		Expect(resp.Code).To(Equal(http.StatusBadRequest), "a reason is required")
		resp = launchReq(http.MethodPost, "/launch-applications/"+unrelated.ID.String()+"/reject", ryID, true, &commands.RejectApplicationCommand{Reason: "the phase was cancelled"})
		Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
		var app entities.LaunchApplication
		Expect(DecodeJSON(resp, &app)).To(Succeed())
		Expect(app.Status).To(Equal(entities.ApplicationStatusRejected))
		resp = launchReq(http.MethodPost, "/launch-applications/"+unrelated.ID.String()+"/reject", ryID, true, &commands.RejectApplicationCommand{Reason: "again"})
		Expect(resp.Code).To(Equal(http.StatusConflict), "final")
	})
})
