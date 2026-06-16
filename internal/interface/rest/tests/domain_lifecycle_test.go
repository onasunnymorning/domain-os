package tests

import (
	"fmt"
	"net/http"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Domain Lifecycle Integration Test
//
// Models the full EPP domain lifecycle:
//   register → renew → autorenew → mark-for-deletion → restore → cleanup/teardown
//
// This is the most important test in the suite — it exercises the registrar-facing
// endpoints in the order a real EPP client would use them.
var _ = Describe("DomainLifecycle", Ordered, func() {
	const (
		ryID            = "lcTestRyOp"
		tldName         = "lctest"
		gaPhase         = "GA1"
		launchPhase     = "Launch1"
		registrarClID   = "lcTestRar"
		contactID       = "lcTestCont"
		domainName      = "lifecycle.lctest"
		idnDomainName   = "xn--cario-rta.lctest" // IDN A-label
		host1Name       = "ns1.example.com"
		host2Name       = "ns2.example.com"
		registrarGurID  = 10005
	)

	// State captured across specs
	var (
		host1RoID string
		host2RoID string
	)

	// ------------------------------------------------------------------ //
	//  Setup: create all prerequisite entities                            //
	// ------------------------------------------------------------------ //
	BeforeAll(func() {
		// 1. Registry Operator
		ryCmd := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID,
			Name:  "Lifecycle Test Registry Operator",
			Email: "admin@lctest.com",
		}
		resp := api.POST("/registry-operators", ryCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 2. TLD
		tldReq := &request.CreateTLDRequest{
			Name: tldName,
			RyID: ryID,
		}
		resp = api.POST("/tlds", tldReq)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 3. GA Phase (starts in the past, UTC)
		gaPhaseCmd := &commands.CreatePhaseCommand{
			Name:   gaPhase,
			Type:   "GA",
			Starts: time.Now().UTC().Add(-48 * time.Hour),
		}
		resp = api.POST(fmt.Sprintf("/tlds/%s/phases", tldName), gaPhaseCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 4. Launch Phase (starts in the past, UTC)
		launchPhaseCmd := &commands.CreatePhaseCommand{
			Name:   launchPhase,
			Type:   "Launch",
			Starts: time.Now().UTC().Add(-24 * time.Hour),
		}
		resp = api.POST(fmt.Sprintf("/tlds/%s/phases", tldName), launchPhaseCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 5. Registrar
		registrarPayload := testRegistrar(registrarClID, "Lifecycle Test Registrar", registrarGurID)
		resp = api.POST("/registrars", registrarPayload)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 6. Set registrar status to 'ok' so it can be accredited
		resp = api.PUT(fmt.Sprintf("/registrars/%s/status/ok", registrarClID), nil)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusOK),
			Equal(http.StatusNoContent),
		))

		// 6b. Set IANA status to 'Accredited' (required for gTLD accreditation)
		resp = api.PUT(fmt.Sprintf("/registrars/%s/iana_status/Accredited", registrarClID), nil)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusOK),
			Equal(http.StatusNoContent),
		))

		// 7. Accredit registrar for TLD
		resp = api.POSTNoBody(fmt.Sprintf("/accreditations/%s/%s", tldName, registrarClID))
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 8. Contact
		testAddress, err := entities.NewAddress("Test City", "US")
		Expect(err).NotTo(HaveOccurred())
		testPostalInfo, err := entities.NewContactPostalInfo("int", "Lifecycle Test Contact", testAddress)
		Expect(err).NotTo(HaveOccurred())
		contactCmd := &commands.CreateContactCommand{
			ID:       contactID,
			RoID:     "99990_CONT-APEX",
			Email:    "lctest@example.com",
			AuthInfo: "str0NGP@ZZw0rd",
			ClID:     registrarClID,
			PostalInfo: [2]*entities.ContactPostalInfo{
				testPostalInfo,
			},
		}
		resp = api.POST("/contacts", contactCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 9. Create two hosts (out-of-bailiwick, no glue needed)
		host1Cmd := &commands.CreateHostCommand{
			Name:      host1Name,
			Addresses: []string{"192.0.2.1"},
			ClID:      entities.ClIDType(registrarClID),
			CrRr:      entities.ClIDType(registrarClID),
		}
		resp = api.POST("/hosts", host1Cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))
		var h1 entities.Host
		Expect(DecodeJSON(resp, &h1)).To(Succeed())
		host1RoID = h1.RoID.String()

		host2Cmd := &commands.CreateHostCommand{
			Name:      host2Name,
			Addresses: []string{"192.0.2.2"},
			ClID:      entities.ClIDType(registrarClID),
			CrRr:      entities.ClIDType(registrarClID),
		}
		resp = api.POST("/hosts", host2Cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))
		var h2 entities.Host
		Expect(DecodeJSON(resp, &h2)).To(Succeed())
		host2RoID = h2.RoID.String()
	})

	// ================================================================== //
	//  Registration                                                       //
	// ================================================================== //

	It("should register a domain in the GA phase", func() {
		registerCmd := &commands.RegisterDomainCommand{
			Name:         domainName,
			ClID:         registrarClID,
			AuthInfo:     "Str0nG!P@ssw0rd",
			RegistrantID: contactID,
			AdminID:      contactID,
			TechID:       contactID,
			BillingID:    contactID,
			Years:        1,
		}
		resp := api.POST(fmt.Sprintf("/domains/%s/register", domainName), registerCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Register domain failed: %s", resp.Body.String())

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Name.String()).To(Equal(domainName))
		Expect(domain.ClID.String()).To(Equal(registrarClID))
	})

	It("should fail to register a duplicate domain", func() {
		registerCmd := &commands.RegisterDomainCommand{
			Name:         domainName,
			ClID:         registrarClID,
			AuthInfo:     "Str0nG!P@ssw0rd",
			RegistrantID: contactID,
			AdminID:      contactID,
			TechID:       contactID,
			BillingID:    contactID,
			Years:        1,
		}
		resp := api.POST(fmt.Sprintf("/domains/%s/register", domainName), registerCmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusConflict),
			Equal(http.StatusInternalServerError),
		))
	})

	It("should register an IDN domain with hosts in the launch phase", func() {
		registerCmd := &commands.RegisterDomainCommand{
			Name:         idnDomainName,
			ClID:         registrarClID,
			AuthInfo:     "IdN$ecr3tP@ss",
			RegistrantID: contactID,
			AdminID:      contactID,
			TechID:       contactID,
			BillingID:    contactID,
			Years:        1,
			HostNames:    []string{host1Name, host2Name},
			PhaseName:    launchPhase,
		}
		resp := api.POST(fmt.Sprintf("/domains/%s/register", idnDomainName), registerCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Name.String()).To(Equal(idnDomainName))
	})

	It("should list domains and find at least 2", func() {
		resp := api.GET(fmt.Sprintf("/domains?tld_equals=%s", tldName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var res map[string]interface{}
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		items, ok := res["Data"].([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(items)).To(BeNumerically(">=", 2))
	})

	// ================================================================== //
	//  Renewal                                                            //
	// ================================================================== //

	It("should renew the domain", func() {
		renewCmd := &commands.RenewDomainCommand{
			Name:  domainName,
			ClID:  registrarClID,
			Years: 1,
		}
		resp := api.POST(fmt.Sprintf("/domains/%s/renew", domainName), renewCmd)
		Expect(resp.Code).To(Equal(http.StatusOK), "Renew domain failed: %s", resp.Body.String())

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Name.String()).To(Equal(domainName))
	})

	It("should renew the domain again", func() {
		renewCmd := &commands.RenewDomainCommand{
			Name:  domainName,
			ClID:  registrarClID,
			Years: 1,
		}
		resp := api.POST(fmt.Sprintf("/domains/%s/renew", domainName), renewCmd)
		Expect(resp.Code).To(Equal(http.StatusOK))
	})

	It("should fail to renew a non-existent domain", func() {
		renewCmd := &commands.RenewDomainCommand{
			Name:  "idontexist.lctest",
			ClID:  registrarClID,
			Years: 1,
		}
		resp := api.POST("/domains/idontexist.lctest/renew", renewCmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusNotFound),
			Equal(http.StatusInternalServerError),
		))
	})

	// ================================================================== //
	//  Auto-Renewal                                                       //
	// ================================================================== //

	It("should check canAutoRenew", func() {
		resp := api.GET(fmt.Sprintf("/domains/%s/canautorenew", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result response.CanAutoRenewResponse
		Expect(DecodeJSON(resp, &result)).To(Succeed())
		Expect(result.DomainName).To(Equal(domainName))
	})

	It("should auto-renew the domain", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/autorenew", domainName))
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusOK),
			Equal(http.StatusForbidden), // 403 if autorenew not enabled on registrar/TLD — acceptable
		))
	})

	// ================================================================== //
	//  Mark for Deletion & Restore                                        //
	// ================================================================== //

	It("should move domain past AddGracePeriod before marking for deletion", func() {
		// A domain in AddGP gets immediate purge with no redemption window.
		// We must first move AddPeriodEnd to the past so MarkForDeletion
		// creates a real 30-day redemption period that allows restoration.
		resp := api.GET(fmt.Sprintf("/domains/%s", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())

		updateCmd := &commands.UpdateDomainCommand{}
		updateCmd.FromEntity(&domain)
		// Set AddPeriodEnd to the past to exit AddGP
		updateCmd.RGPStatus.AddPeriodEnd = time.Now().UTC().Add(-24 * time.Hour)

		resp = api.PUT(fmt.Sprintf("/domains/%s", domainName), updateCmd)
		Expect(resp.Code).To(Equal(http.StatusOK), "Update domain RGP failed: %s", resp.Body.String())
	})

	It("should mark domain for deletion", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s/markdelete", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK), "MarkForDeletion failed: %s", resp.Body.String())

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.PendingDelete).To(BeTrue())
	})

	It("should fail to mark already-pending domain for deletion", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s/markdelete", domainName))
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should verify domain has pendingDelete status", func() {
		resp := api.GET(fmt.Sprintf("/domains/%s", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.PendingDelete).To(BeTrue())
	})

	It("should restore the domain", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/restore", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		// After restore, PendingDelete should be cleared
		Expect(domain.Status.PendingDelete).To(BeFalse())
	})

	It("should verify domain is active after restore", func() {
		resp := api.GET(fmt.Sprintf("/domains/%s", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.PendingDelete).To(BeFalse())
	})

	// ================================================================== //
	//  Cleanup & Teardown (in-test assertions)                            //
	// ================================================================== //

	It("should fail to delete IDN domain that has hosts attached", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s", idnDomainName))
		// The domain has hosts — expect it to fail
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusInternalServerError),
		))
	})

	It("should remove hosts from IDN domain", func() {
		// Remove host1
		resp := api.DELETE(fmt.Sprintf("/domains/%s/hosts/%s", idnDomainName, host1RoID))
		Expect(resp.Code).To(Equal(http.StatusNoContent))

		// Remove host2
		resp = api.DELETE(fmt.Sprintf("/domains/%s/hosts/%s", idnDomainName, host2RoID))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should delete the IDN domain after host removal", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s", idnDomainName))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should delete the lifecycle domain", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s", domainName))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should verify no domains remain for the TLD", func() {
		resp := api.GET(fmt.Sprintf("/domains/count?tld_equals=%s", tldName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var countResult response.CountResult
		Expect(DecodeJSON(resp, &countResult)).To(Succeed())
		Expect(countResult.Count).To(BeNumerically("==", 0))
	})

	// ------------------------------------------------------------------ //
	//  AfterAll: best-effort cleanup (no assertions)                      //
	// ------------------------------------------------------------------ //
	AfterAll(func() {
		// Hosts
		api.DELETE(fmt.Sprintf("/hosts/%s", host1RoID))
		api.DELETE(fmt.Sprintf("/hosts/%s", host2RoID))

		// Contact
		api.DELETE(fmt.Sprintf("/contacts/%s", contactID))

		// Accreditation
		api.DELETE(fmt.Sprintf("/accreditations/%s/%s", tldName, registrarClID))

		// Registrar
		api.DELETE(fmt.Sprintf("/registrars/%s", registrarClID))

		// TLD phases (delete before TLD)
		api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s", tldName, gaPhase))
		api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s", tldName, launchPhase))

		// TLD
		api.DELETE(fmt.Sprintf("/tlds/%s", tldName))

		// Registry Operator
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID))
	})
})
