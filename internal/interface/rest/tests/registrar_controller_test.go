package tests

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
)

var _ = ginkgo.Describe("RegistrarController", func() {
	var (
		router               *gin.Engine
		registrarService     interfaces.RegistrarService
		ianaRegistrarService interfaces.IANARegistrarService
		registrarController  *rest.RegistrarController
		createdID            string
	)

	ginkgo.BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		router = gin.New()
		gormDB, err := getTestDB()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		registrarRepo := postgres.NewGormRegistrarRepository(gormDB)
		registrarService = services.NewRegistrarService(registrarRepo)

		ianaRepo := postgres.NewIANARegistrarRepository(gormDB)
		ianaRegistrarService = services.NewIANARegistrarService(ianaRepo)

		registrarController = rest.NewRegistrarController(router, registrarService, ianaRegistrarService)
		_ = registrarController
	})

	ginkgo.AfterEach(func() {
		// Cleanup test data
	})

	ginkgo.Context("when the registrar does not exist", func() {
		ginkgo.It("should return 404 NOT FOUND for GetByClID", func() {
			nonExistentClID := "nonexistent-clid"
			req, _ := http.NewRequest(http.MethodGet, "/registrars/"+nonExistentClID, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			gomega.Expect(resp.Code).To(gomega.Equal(http.StatusNotFound))
		})
	})

	ginkgo.Context("when creating a new registrar", func() {
		ginkgo.It("should return 200 OK and create the registrar", func() {
			filePath := "registrar_create.json"
			payloadBytes, _ := os.ReadFile(filePath)
			req, _ := http.NewRequest(http.MethodPost, "/registrars", bytes.NewReader(payloadBytes))
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			res := &commands.CreateRegistrarCommandResult{}
			_ = json.NewDecoder(resp.Body).Decode(res)
			createdID = res.Result.ClID.String()
			gomega.Expect(resp.Code).To(gomega.Equal(http.StatusOK))
		})
	})

	ginkgo.Context("listing registrars", func() {
		ginkgo.It("should return 200 OK with a list of registrars", func() {
			// Your test code here for GET /registrars
		})
	})

	ginkgo.Context("deleting a registrar", func() {
		ginkgo.It("should return 204 NO CONTENT when deleting an existing registrar", func() {
			req, _ := http.NewRequest(http.MethodDelete, "/registrars/"+createdID, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			gomega.Expect(resp.Code).To(gomega.Equal(http.StatusNoContent))
		})
	})

	ginkgo.Context("creating a registrar by GurID", func() {
		ginkgo.It("should return 200 OK and create the registrar from GurID", func() {
			// Your test code here for POST /registrars/:gurid
		})
	})
})
