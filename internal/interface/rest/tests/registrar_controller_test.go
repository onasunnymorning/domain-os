package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RegistrarController", func() {
	Context("Managing a registrar", func() {
		// Initialize your router
		gin.SetMode(gin.TestMode)
		router := gin.New()

		// Initialize your database connection
		db, err := getTestDB()
		Expect(err).NotTo(HaveOccurred())

		registrarRepo := postgres.NewGormRegistrarRepository(db)
		registrarService := services.NewRegistrarService(registrarRepo)

		ianaRepo := postgres.NewIANARegistrarRepository(db)
		ianaRegistrarService := services.NewIANARegistrarService(ianaRepo)

		registrarController := rest.NewRegistrarController(router, registrarService, ianaRegistrarService, MockGinHandler())
		Expect(registrarController).NotTo(BeNil())

		var createdID string
		// Define the registrar payload directly
		registrarPayload := testRegistrar("testRegistrarID", "Test Registrar Name")

		It("should create a new registrar and return its ID", func() {

			// Marshal the payload to JSON
			payloadBytes, err := json.Marshal(registrarPayload)
			Expect(err).NotTo(HaveOccurred())

			// Send the request to create a new registrar
			req, err := http.NewRequest(http.MethodPost, "/registrars", bytes.NewReader(payloadBytes))
			Expect(err).NotTo(HaveOccurred())

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			var res entities.Registrar
			err = json.NewDecoder(resp.Body).Decode(&res)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Code).To(Equal(http.StatusCreated))

			createdID = res.ClID.String()
			Expect(createdID).NotTo(BeEmpty())
		})

		It("should get the created registrar by its ID and assert its properties", func() {
			// Assuming `createdID` contains the ID of the previously created registrar
			Expect(createdID).NotTo(BeEmpty(), "The registrar ID should not be empty. Ensure the registrar creation test passed.")

			req, err := http.NewRequest(http.MethodGet, "/registrars/"+createdID, nil)
			Expect(err).NotTo(HaveOccurred())

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			// Ensure we received an OK response
			Expect(resp.Code).To(Equal(http.StatusOK))

			var retrievedRegistrar entities.Registrar
			err = json.NewDecoder(resp.Body).Decode(&retrievedRegistrar)
			Expect(err).NotTo(HaveOccurred())

			Expect(retrievedRegistrar.ClID.String()).To(Equal(registrarPayload.ClID))
			Expect(retrievedRegistrar.Name).To(Equal(registrarPayload.Name))
			Expect(retrievedRegistrar.Email).To(Equal(registrarPayload.Email))
			Expect(retrievedRegistrar.PostalInfo).To(HaveLen(2))
		})

		It("should list all registrars", func() {
			req, err := http.NewRequest(http.MethodGet, "/registrars", nil)
			Expect(err).NotTo(HaveOccurred())

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusOK))
			res := response.ListItemResult{}
			filter := queries.ListRegistrarsFilter{}
			res.Meta.Filter = &filter
			err = json.Unmarshal(resp.Body.Bytes(), &res)
			Expect(err).NotTo(HaveOccurred())
			itemSlice, ok := res.Data.([]interface{})
			Expect(ok).To(BeTrue())
			Expect(len(itemSlice)).To(BeNumerically(">", 0))
		})

		It("should delete the created registrar", func() {
			Expect(createdID).NotTo(BeEmpty(), "The registrar ID should not be empty. Ensure the registrar creation test passed.")

			req, err := http.NewRequest(http.MethodDelete, "/registrars/"+createdID, nil)
			Expect(err).NotTo(HaveOccurred())

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusNoContent))
		})

	})

})

func testRegistrar(clid string, name string) *commands.CreateRegistrarCommand {
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
		GurID:       12345,
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
