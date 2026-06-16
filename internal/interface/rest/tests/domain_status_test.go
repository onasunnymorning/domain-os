package tests

import (
	"fmt"
	"net/http"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Domain Status Integration Test
//
// Tests all domain status transitions and conflict rules via the
// POST /domains/:name/status/:status (set) and
// DELETE /domains/:name/status/:status (unset) endpoints.
//
// Key validation rules exercised:
//   - Cannot manually set 'ok' or 'inactive' (they are automatic)
//   - Cannot set pendingDelete when clientDeleteProhibited or serverDeleteProhibited is set
//   - SetStatus/UnSetStatus return the updated domain entity
var _ = Describe("DomainStatus", Ordered, func() {
	const (
		ryID          = "dsTestRyOp"
		tldName       = "dstest"
		registrarClID = "dsTestRar"
		registrarGurID = 10011
		contactID     = "dsTestCont"
		domainName    = "status.dstest"
	)

	// ------------------------------------------------------------------ //
	//  Setup: create prerequisite entities                                //
	// ------------------------------------------------------------------ //
	BeforeAll(func() {
		// 1. Registry Operator
		ryCmd := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID,
			Name:  "Domain Status Test Registry Operator",
			Email: "admin@dstest.com",
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

		// 3. Registrar
		registrarPayload := testRegistrar(registrarClID, "Domain Status Test Registrar", registrarGurID)
		resp = api.POST("/registrars", registrarPayload)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 4. Contact
		testAddress, err := entities.NewAddress("Test City", "US")
		Expect(err).NotTo(HaveOccurred())
		testPostalInfo, err := entities.NewContactPostalInfo("int", "DS Test Contact", testAddress)
		Expect(err).NotTo(HaveOccurred())
		contactCmd := &commands.CreateContactCommand{
			ID:       contactID,
			RoID:     "10011_CONT-APEX",
			Email:    "dstest@example.com",
			AuthInfo: "str0NGP@ZZw0rd",
			ClID:     registrarClID,
			PostalInfo: [2]*entities.ContactPostalInfo{
				testPostalInfo,
			},
		}
		resp = api.POST("/contacts", contactCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 5. Domain via admin import (POST /domains)
		domainCmd := &commands.CreateDomainCommand{
			Name:         domainName,
			ClID:         registrarClID,
			AuthInfo:     "St@tusT3st!Pwd",
			RegistrantID: contactID,
			ExpiryDate:   time.Now().UTC().Add(365 * 24 * time.Hour),
			RenewedYears: 1,
		}
		resp = api.POST("/domains", domainCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Name.String()).To(Equal(domainName))
	})

	// ================================================================== //
	//  Automatic statuses cannot be set manually                          //
	// ================================================================== //

	It("should fail to set status ok (ok is automatic)", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/status/ok", domainName))
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should fail to set status INACTIVE (case-sensitive, INACTIVE not in valid list)", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/status/INACTIVE", domainName))
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusNotFound),
		))
	})

	It("should fail to set status inactive (inactive is automatic)", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/status/inactive", domainName))
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	// ================================================================== //
	//  Setting and unsetting pending statuses                             //
	// ================================================================== //

	It("should set status pendingRestore", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/status/pendingRestore", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.PendingRestore).To(BeTrue())
	})

	It("should unset status pendingRestore", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s/status/pendingRestore", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.PendingRestore).To(BeFalse())
	})

	It("should set status pendingDelete", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/status/pendingDelete", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.PendingDelete).To(BeTrue())
	})

	It("should set status pendingDelete again (idempotent)", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/status/pendingDelete", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.PendingDelete).To(BeTrue())
	})

	It("should unset status pendingDelete", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s/status/pendingDelete", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.PendingDelete).To(BeFalse())
	})

	// ================================================================== //
	//  Conflict rule: pendingDelete blocked by clientDeleteProhibited     //
	// ================================================================== //

	It("should set status clientDeleteProhibited", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/status/clientDeleteProhibited", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.ClientDeleteProhibited).To(BeTrue())
	})

	It("should fail to set pendingDelete when clientDeleteProhibited is set", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/status/pendingDelete", domainName))
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should unset status clientDeleteProhibited", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s/status/clientDeleteProhibited", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.ClientDeleteProhibited).To(BeFalse())
	})

	// ================================================================== //
	//  Hold statuses                                                      //
	// ================================================================== //

	It("should set status clientHold", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/status/clientHold", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.ClientHold).To(BeTrue())
	})

	It("should unset status clientHold", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s/status/clientHold", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.ClientHold).To(BeFalse())
	})

	It("should set status serverHold", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/status/serverHold", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.ServerHold).To(BeTrue())
	})

	It("should unset status serverHold", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s/status/serverHold", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		Expect(DecodeJSON(resp, &domain)).To(Succeed())
		Expect(domain.Status.ServerHold).To(BeFalse())
	})

	// ================================================================== //
	//  Error cases                                                        //
	// ================================================================== //

	It("should fail to set an invalid status name", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/status/totallyBogusStatus", domainName))
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusNotFound),
		))
	})

	It("should return 404 when setting status on a non-existent domain", func() {
		resp := api.POSTNoBody("/domains/nosuchdomain.dstest/status/clientHold")
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	// ------------------------------------------------------------------ //
	//  AfterAll: best-effort cleanup (no assertions)                      //
	// ------------------------------------------------------------------ //
	AfterAll(func() {
		api.DELETE(fmt.Sprintf("/domains/%s", domainName))
		api.DELETE(fmt.Sprintf("/contacts/%s", contactID))
		api.DELETE(fmt.Sprintf("/registrars/%s", registrarClID))
		api.DELETE(fmt.Sprintf("/tlds/%s", tldName))
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID))
	})
})
