package tests

import (
	"fmt"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RegistrarController", Ordered, func() {
	var createdID string
	registrarPayload := testRegistrar("testRegistrarID", "Test Registrar Name", 10001)

	It("should create a new registrar and return its ID", func() {
		resp := api.POST("/registrars", registrarPayload)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		var res entities.Registrar
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		createdID = res.ClID.String()
		Expect(createdID).NotTo(BeEmpty())
	})

	It("should get the created registrar by its ID and assert its properties", func() {
		Expect(createdID).NotTo(BeEmpty(), "The registrar ID should not be empty")

		resp := api.GET(fmt.Sprintf("/registrars/%s", createdID))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var retrievedRegistrar entities.Registrar
		err := DecodeJSON(resp, &retrievedRegistrar)
		Expect(err).NotTo(HaveOccurred())
		Expect(retrievedRegistrar.ClID.String()).To(Equal(registrarPayload.ClID))
		Expect(retrievedRegistrar.Name).To(Equal(registrarPayload.Name))
		Expect(retrievedRegistrar.Email).To(Equal(registrarPayload.Email))
		Expect(retrievedRegistrar.PostalInfo).To(HaveLen(2))
	})

	It("should list all registrars", func() {
		resp := api.GET("/registrars")
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListRegistrarsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(BeNumerically(">", 0))
	})

	It("should delete the created registrar", func() {
		Expect(createdID).NotTo(BeEmpty(), "The registrar ID should not be empty")

		resp := api.DELETE(fmt.Sprintf("/registrars/%s", createdID))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})
})

// testRegistrar is a shared helper that creates a valid CreateRegistrarCommand.
func testRegistrar(clid string, name string, gurID int) *commands.CreateRegistrarCommand {
	return &commands.CreateRegistrarCommand{
		ClID:  clid,
		Name:  name,
		Email: "contact@example.com",
		PostalInfo: [2]*entities.RegistrarPostalInfo{
			{
				Type: "loc",
				Address: &entities.Address{
					Street1:       "Boulnes 2545",
					Street2:       "Piso 8",
					Street3:       "Portero",
					City:          "Buenos Aires",
					StateProvince: "Palermo SOHO",
					PostalCode:    "EN234Z",
					CountryCode:   "AR",
				},
			},
			{
				Type: "int",
				Address: &entities.Address{
					Street1:       "Boulnes 2545",
					Street2:       "Piso 8",
					Street3:       "Portero",
					City:          "Buenos Aires",
					StateProvince: "Palermo SOHO",
					PostalCode:    "EN234Z",
					CountryCode:   "AR",
				},
			},
		},
		GurID:       gurID,
		Voice:       "+1.5555555555",
		Fax:         "+1.5555555556",
		URL:         "https://example.com",
		RdapBaseURL: "https://rdap.example.com",
		WhoisInfo: &entities.WhoisInfo{
			Name: "whois.apex.domains",
			URL:  "https://apex.domains/whois",
		},
	}
}
