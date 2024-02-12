package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
)

var _ = Describe("RegistrarController", func() {
	var (
		router *gin.Engine
		gormDB *gorm.DB
	)

	gin.SetMode(gin.TestMode)
	router = gin.New()
	gormDB, err := getTestDB()
	Expect(err).NotTo(HaveOccurred())

	var (
		registrarService     interfaces.RegistrarService
		ianaRegistrarService interfaces.IANARegistrarService
		registrarController  *rest.RegistrarController
	)

	registrarRepo := postgres.NewGormRegistrarRepository(gormDB)
	registrarService = services.NewRegistrarService(registrarRepo)

	ianaRepo := postgres.NewIANARegistrarRepository(gormDB)
	ianaRegistrarService = services.NewIANARegistrarService(ianaRepo)

	registrarController = rest.NewRegistrarController(router, registrarService, ianaRegistrarService)
	_ = registrarController

	Context("Managing a registrar", func() {
		var createdID string
		// Define the registrar payload directly
		registrarPayload := entities.Registrar{
			ClID:  "exampleClID",
			Name:  "Example Registrar Name",
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
			WhoisInfo: entities.WhoisInfo{
				Name: "whois.apex.domains",
				URL:  "https://apex.domains/whois",
			},
		}

		It("should create a new registrar and return its ID", func() {

			// Marshal the payload to JSON
			payloadBytes, err := json.Marshal(registrarPayload)
			Expect(err).NotTo(HaveOccurred())

			// Send the request to create a new registrar
			req, err := http.NewRequest(http.MethodPost, "/registrars", bytes.NewReader(payloadBytes))
			Expect(err).NotTo(HaveOccurred())

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			var res commands.CreateRegistrarCommandResult
			err = json.NewDecoder(resp.Body).Decode(&res)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Code).To(Equal(http.StatusOK))

			createdID = res.Result.ClID.String()
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

			Expect(retrievedRegistrar.ClID).To(Equal(registrarPayload.ClID))
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
			var res []entities.Registrar
			err = json.Unmarshal(resp.Body.Bytes(), &res)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(res)).To(BeNumerically(">", 0))
		})

		It("should delete the created registrar", func() {
			Expect(createdID).NotTo(BeEmpty(), "The registrar ID should not be empty. Ensure the registrar creation test passed.")

			req, err := http.NewRequest(http.MethodDelete, "/registrars/"+createdID, nil)
			Expect(err).NotTo(HaveOccurred())

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusNoContent))
		})

		It("should NOT create a new registrar by non existing GurID", func() {
			reqBody := request.CreateRegistrarFromGurIDRequest{
				Email: "contact@example.com",
			}
			payloadBytes, err := json.Marshal(reqBody)
			Expect(err).NotTo(HaveOccurred())
			gurid := "12345"
			// Send the request to create a new registrar
			req, err := http.NewRequest(http.MethodPost, "/registrars/"+gurid, bytes.NewReader(payloadBytes))
			Expect(err).NotTo(HaveOccurred())

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			var res commands.CreateRegistrarCommandResult
			err = json.NewDecoder(resp.Body).Decode(&res)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Code).To(Equal(http.StatusBadRequest))
		})
	})

})
