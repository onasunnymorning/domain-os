package tests

import (
	"fmt"
	"net/http"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DomainController", Ordered, func() {
	const (
		ryID          = "domTestRyOp"
		tldName       = "domtest"
		registrarClID = "domTestRegistrar"
		contactID     = "domTestContact"
		domainName    = "example.domtest"
	)

	domainCmd := &commands.CreateDomainCommand{
		Name:         domainName,
		ClID:         registrarClID,
		AuthInfo:     "domainAuth123!",
		RegistrantID: contactID,
		ExpiryDate:   time.Now().Add(365 * 24 * time.Hour),
		RenewedYears: 1,
	}

	BeforeAll(func() {
		// 1. Registry Operator
		ryCmd := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID,
			Name:  "Domain Test Registry Operator",
			Email: "admin@domtest.com",
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

		// 3. Phase (GA, starts in the past, must be UTC)
		phaseCmd := &commands.CreatePhaseCommand{
			Name:   "GA1",
			Type:   "GA",
			Starts: time.Now().UTC().Add(-24 * time.Hour),
		}
		resp = api.POST(fmt.Sprintf("/tlds/%s/phases", tldName), phaseCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 4. Registrar
		registrarPayload := testRegistrar(registrarClID, "Domain Test Registrar", 10004)
		resp = api.POST("/registrars", registrarPayload)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 5. Contact
		testAddress, err := entities.NewAddress("Test City", "US")
		Expect(err).NotTo(HaveOccurred())
		testPostalInfo, err := entities.NewContactPostalInfo("int", "Domain Test Contact", testAddress)
		Expect(err).NotTo(HaveOccurred())
		contactCmd := &commands.CreateContactCommand{
			ID:       contactID,
			RoID:     "99999_CONT-APEX",
			Email:    "domtest@example.com",
			AuthInfo: "str0NGP@ZZw0rd",
			ClID:     registrarClID,
			PostalInfo: [2]*entities.ContactPostalInfo{
				testPostalInfo,
			},
		}
		resp = api.POST("/contacts", contactCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	It("should create a domain via admin import", func() {
		resp := api.POST("/domains", domainCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		var domain entities.Domain
		err := DecodeJSON(resp, &domain)
		Expect(err).NotTo(HaveOccurred())
		Expect(domain.Name.String()).To(Equal(domainName))
		Expect(domain.ClID.String()).To(Equal(registrarClID))
	})

	It("should get the domain by name", func() {
		resp := api.GET(fmt.Sprintf("/domains/%s", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		err := DecodeJSON(resp, &domain)
		Expect(err).NotTo(HaveOccurred())
		Expect(domain.Name.String()).To(Equal(domainName))
		Expect(domain.ClID.String()).To(Equal(registrarClID))
	})

	It("should list domains", func() {
		resp := api.GET("/domains")
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListDomainsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(BeNumerically(">", 0))
	})

	It("should count domains", func() {
		resp := api.GET("/domains/count")
		Expect(resp.Code).To(Equal(http.StatusOK))

		var countResult response.CountResult
		err := DecodeJSON(resp, &countResult)
		Expect(err).NotTo(HaveOccurred())
		Expect(countResult.Count).To(BeNumerically(">", 0))
	})

	It("should check domain availability for a taken domain", func() {
		resp := api.GET(fmt.Sprintf("/domains/%s/available", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result queries.DomainCheckResult
		err := DecodeJSON(resp, &result)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Available).To(BeFalse())
	})

	It("should set clientHold status on the domain", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/domains/%s/status/clientHold", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		err := DecodeJSON(resp, &domain)
		Expect(err).NotTo(HaveOccurred())
		Expect(domain.Status.ClientHold).To(BeTrue())
	})

	It("should unset clientHold status on the domain", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s/status/clientHold", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var domain entities.Domain
		err := DecodeJSON(resp, &domain)
		Expect(err).NotTo(HaveOccurred())
		Expect(domain.Status.ClientHold).To(BeFalse())
	})

	It("should list events for the domain", func() {
		resp := api.GET(fmt.Sprintf("/domains/%s/events", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var events []entities.DomainEvent
		err := DecodeJSON(resp, &events)
		Expect(err).NotTo(HaveOccurred())
		Expect(events).NotTo(BeEmpty())

		// Verify event properties
		Expect(events[0].Subject).To(Equal(domainName))
		Expect(events[0].Source).To(Equal("domain-os/api"))
	})

	It("should delete the domain", func() {
		resp := api.DELETE(fmt.Sprintf("/domains/%s", domainName))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should not find the deleted domain", func() {
		resp := api.GET(fmt.Sprintf("/domains/%s", domainName))
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	AfterAll(func() {
		// Best-effort cleanup in reverse order of creation
		api.DELETE(fmt.Sprintf("/contacts/%s", contactID))
		api.DELETE(fmt.Sprintf("/registrars/%s", registrarClID))
		api.DELETE(fmt.Sprintf("/tlds/%s", tldName))
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID))
	})
})
