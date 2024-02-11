package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"gorm.io/gorm"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
)

var _ = ginkgo.Describe("RegistrarController", func() {
	var (
		router *gin.Engine
		gormDB *gorm.DB
	)

	gin.SetMode(gin.TestMode)
	router = gin.New()
	gormDB, err := getTestDB()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

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

	ginkgo.Context("Managing a registrar", func() {
		var createdID string

		ginkgo.It("should create a new registrar and return its ID", func() {
			filePath := "registrar_create.json"
			payloadBytes, err := os.ReadFile(filePath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequest(http.MethodPost, "/registrars", bytes.NewReader(payloadBytes))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			var res commands.CreateRegistrarCommandResult
			err = json.NewDecoder(resp.Body).Decode(&res)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resp.Code).To(gomega.Equal(http.StatusOK))

			createdID = res.Result.ClID.String()
			gomega.Expect(createdID).NotTo(gomega.BeEmpty())
		})

		ginkgo.It("should get the created registrar by its ID", func() {
			gomega.Expect(createdID).NotTo(gomega.BeEmpty(), "The registrar ID should not be empty. Ensure the registrar creation test passed.")

			req, err := http.NewRequest(http.MethodGet, "/registrars/"+createdID, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			gomega.Expect(resp.Code).To(gomega.Equal(http.StatusOK))
		})

		ginkgo.It("should list all registrars", func() {
			req, err := http.NewRequest(http.MethodGet, "/registrars", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			gomega.Expect(resp.Code).To(gomega.Equal(http.StatusOK))
			res := []entities.Registrar{}
			err = json.Unmarshal(resp.Body.Bytes(), &res)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(len(res)).To(gomega.BeNumerically(">", 0))
		})

		ginkgo.It("should delete the created registrar", func() {
			gomega.Expect(createdID).NotTo(gomega.BeEmpty(), "The registrar ID should not be empty. Ensure the registrar creation test passed.")

			req, err := http.NewRequest(http.MethodDelete, "/registrars/"+createdID, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			gomega.Expect(resp.Code).To(gomega.Equal(http.StatusNoContent))
		})
	})

})
