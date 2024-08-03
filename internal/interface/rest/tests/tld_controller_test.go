package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
)

var _ = ginkgo.Describe("TLDController", func() {
	var (
		router        *gin.Engine
		tldService    interfaces.TLDService
		domService    interfaces.DomainService
		tldController *rest.TLDController
		tempTLDName   string
	)

	ginkgo.BeforeEach(func() {
		// Initialize your router
		gin.SetMode(gin.TestMode)
		router = gin.New()
		router.Use(rest.PublishEvent(nil, ""))
		db, err := getTestDB()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		tldRepo := postgres.NewGormTLDRepo(db)
		dnsRecRepo := postgres.NewGormDNSRecordRepository(db)
		tldService = services.NewTLDService(tldRepo, dnsRecRepo)
		tldController = rest.NewTLDController(router, tldService, domService)
		_ = tldController
	})

	ginkgo.BeforeEach(func() {
		tempTLDName = "mytesttld" // Define a unique TLD name for each test run if needed

		// Create a TLD
		tldCreatePayload := map[string]interface{}{
			"name": tempTLDName,
		}
		payloadBytes, _ := json.Marshal(tldCreatePayload)
		createReq, _ := http.NewRequest(http.MethodPost, "/tlds", bytes.NewReader(payloadBytes))
		createReq.Header.Set("Content-Type", "application/json")
		createResp := httptest.NewRecorder()
		router.ServeHTTP(createResp, createReq)
		gomega.Expect(createResp.Code).To(gomega.Equal(http.StatusCreated))
	})

	ginkgo.AfterEach(func() {
		// Delete the TLD
		deleteReq, _ := http.NewRequest(http.MethodDelete, "/tlds/"+tempTLDName, nil)
		deleteResp := httptest.NewRecorder()
		router.ServeHTTP(deleteResp, deleteReq)
		gomega.Expect(deleteResp.Code).To(gomega.Equal(http.StatusNoContent))
	})

	ginkgo.Context("when the TLD does not exist", func() {
		ginkgo.It("should return 404 NOT FOUND", func() {
			unknownTLDName := "nonexistent-tld"
			req, _ := http.NewRequest(http.MethodGet, "/tlds/"+unknownTLDName, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			gomega.Expect(resp.Code).To(gomega.Equal(http.StatusNotFound))
		})
	})

	ginkgo.Context("when the TLD exists", func() {
		ginkgo.It("should return 200 StatusOK with correct data", func() {
			// Retrieve the TLD
			getReq, _ := http.NewRequest(http.MethodGet, "/tlds/"+tempTLDName, nil)
			getResp := httptest.NewRecorder()
			router.ServeHTTP(getResp, getReq)
			gomega.Expect(getResp.Code).To(gomega.Equal(http.StatusOK))

			var retrievedTLD entities.TLD
			err := json.Unmarshal(getResp.Body.Bytes(), &retrievedTLD)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(retrievedTLD.Name.String()).To(gomega.Equal(tempTLDName))
		})
	})
})
