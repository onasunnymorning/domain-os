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

// Domain Extended Integration Test
//
// Extends domain CRUD with:
//   - Update domain (PUT /domains/:name) — change authInfo
//   - Attach host to domain (POST /domains/:name/hostname/:hostName)
//   - List domain hosts (verify via GET /domains/:name)
//   - Remove host from domain (DELETE /domains/:name/hostname/:hostName)
//   - Search domains (GET /domains?name_like=...)
//   - List purgeable domains (GET /domains/purgeable)
var _ = Describe("DomainExtended", Ordered, func() {
	const (
		ryID          = "dxTestRyOp"
		tldName       = "dxtest"
		gaPhase       = "GA1"
		registrarClID = "dxTestRar"
		registrarGurID = 10013
		contactID     = "dxTestCont"
		domainName    = "extended.dxtest"
		host1Name     = "ns1.dxext.com"
		host2Name     = "ns2.dxext.com"
	)

	// Shared state across specs
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
			Name:  "Domain Extended Test Registry Operator",
			Email: "admin@dxtest.com",
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

		// 4. Registrar
		registrarPayload := testRegistrar(registrarClID, "Domain Extended Test Registrar", registrarGurID)
		resp = api.POST("/registrars", registrarPayload)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 5. Set registrar status to 'ok'
		resp = api.PUT(fmt.Sprintf("/registrars/%s/status/ok", registrarClID), nil)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusOK),
			Equal(http.StatusNoContent),
		))

		// 6. Set IANA status to 'Accredited'
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
		testPostalInfo, err := entities.NewContactPostalInfo("int", "DX Test Contact", testAddress)
		Expect(err).NotTo(HaveOccurred())
		contactCmd := &commands.CreateContactCommand{
			ID:       contactID,
			RoID:     "10013_CONT-APEX",
			Email:    "dxtest@example.com",
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
			Addresses: []string{"192.0.2.10"},
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
			Addresses: []string{"192.0.2.11"},
			ClID:      entities.ClIDType(registrarClID),
			CrRr:      entities.ClIDType(registrarClID),
		}
		resp = api.POST("/hosts", host2Cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))
		var h2 entities.Host
		Expect(DecodeJSON(resp, &h2)).To(Succeed())
		host2RoID = h2.RoID.String()

		// 10. Register domain via EPP-style endpoint
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
		resp = api.POST(fmt.Sprintf("/domains/%s/register", domainName), registerCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Register domain failed: %s", resp.Body.String())
	})

	// ================================================================== //
	//  Update Domain                                                      //
	// ================================================================== //

	It("should update domain authInfo via PUT", func() {
		// GET current domain
		resp := api.GET(fmt.Sprintf("/domains/%s", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())

		// Build update command from entity and modify authInfo
		updateCmd := &commands.UpdateDomainCommand{}
		updateCmd.FromEntity(&domain)
		updateCmd.AuthInfo = "N3wStr0nG!P@ss"

		resp = api.PUT(fmt.Sprintf("/domains/%s", domainName), updateCmd)
		Expect(resp.Code).To(Equal(http.StatusOK), "Update domain failed: %s", resp.Body.String())

		var updated entities.Domain
		Expect(DecodeJSON(resp, &updated)).To(Succeed())
		Expect(updated.AuthInfo.String()).To(Equal("N3wStr0nG!P@ss"))
	})

	It("should verify the update took effect", func() {
		resp := api.GET(fmt.Sprintf("/domains/%s", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.AuthInfo.String()).To(Equal("N3wStr0nG!P@ss"))
	})

	// ================================================================== //
	//  Host Attachment                                                    //
	// ================================================================== //

	It("should attach host to domain by hostname", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/hostname/%s", domainName, host1Name))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should verify domain has the attached host via GET", func() {
		resp := api.GET(fmt.Sprintf("/domains/%s", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		// Domain should have at least one host
		Expect(domain.Hosts).NotTo(BeEmpty())
	})

	It("should remove host from domain by hostname", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s/hostname/%s", domainName, host1Name))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	// ================================================================== //
	//  Search & List                                                      //
	// ================================================================== //

	It("should search domains by name_like", func() {
		resp := api.GET(fmt.Sprintf("/domains?name_like=%s", "extended"))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var res map[string]interface{}
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		items, ok := res["Data"].([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(items)).To(BeNumerically(">=", 1))
	})

	It("should list purgeable domains endpoint", func() {
		resp := api.GET("/domains/purgeable")
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		// We don't assert count since our domain is not purgeable; just check endpoint works
	})

	// ================================================================== //
	//  Cleanup                                                            //
	// ================================================================== //

	It("should delete the domain", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s", domainName))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should verify domain is gone", func() {
		resp := api.GET(fmt.Sprintf("/domains/%s", domainName))
		Expect(resp.Code).To(Equal(http.StatusNotFound))
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

		// Phase
		api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s", tldName, gaPhase))

		// TLD
		api.DELETE(fmt.Sprintf("/tlds/%s", tldName))

		// Registry Operator
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID))
	})
})
