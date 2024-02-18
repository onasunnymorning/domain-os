package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContactController", func() {
	var (
		router            *gin.Engine
		contactController *rest.ContactController
		registrarService  *services.RegistrarService
	)

	BeforeEach(func() {
		// Initialize your router
		router = gin.New()

		registrarRepo := postgres.NewGormRegistrarRepository(tx)
		registrarService = services.NewRegistrarService(registrarRepo)
		_ = registrarService

		// Initialize your repository and service
		contactRepo := postgres.NewContactRepository(tx)
		contactService := services.NewContactService(contactRepo)

		// Initialize and register your controller with the router
		contactController = rest.NewContactController(router, contactService)
		Expect(contactController).NotTo(BeNil())
	})

	Describe("Managing contacts", func() {
		registrarClid := "exampleCLID"
		testContact := &commands.CreateContactCommand{
			ID:       registrarClid,
			RoID:     "12345_CONT-APEX",
			Email:    "jon@doe.com",
			AuthInfo: "str0NGP@ZZw0rd",
		}

		var createdContact entities.Contact

		It("should successfully create a contact", func() {
			registrarPayload := testRegistrar(registrarClid)
			cmdResult, err := registrarService.Create(context.Background(), registrarPayload)
			Expect(err).NotTo(HaveOccurred())
			Expect(cmdResult).NotTo(BeNil())

			payloadBytes, _ := json.Marshal(testContact)

			req, _ := http.NewRequest(http.MethodPost, "/contacts", bytes.NewReader(payloadBytes))
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			// todo
			fmt.Println("+++++", resp.Body.String(), "----")
			Expect(resp.Code).To(Equal(http.StatusCreated))

			err = json.NewDecoder(resp.Body).Decode(&createdContact)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdContact.ID).To(Equal(testContact.ID))
			Expect(createdContact.RoID).To(Equal(testContact.RoID))
			Expect(createdContact.Email).To(Equal(testContact.Email))
			Expect(createdContact.AuthInfo).To(Equal(testContact.AuthInfo))
		})

		It("should not create a contact with an existing ID", func() {
			payloadBytes, _ := json.Marshal(testContact)

			req, _ := http.NewRequest(http.MethodPost, "/contacts", bytes.NewReader(payloadBytes))
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusBadRequest))
		})

		It("should not create a contact with an invalid email", func() {
			invalidContact := &commands.CreateContactCommand{
				ID:       "contactID102",
				RoID:     "12345_CONT-APEX",
				Email:    "invalid-email",
				AuthInfo: "str0NGP@ZZw0rd",
			}
			payloadBytes, _ := json.Marshal(invalidContact)

			req, _ := http.NewRequest(http.MethodPost, "/contacts", bytes.NewReader(payloadBytes))
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusBadRequest))
		})

		It("should retrieve a contact by ID", func() {
			req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/contacts/%s", testContact.ID), nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusOK))

			var retrievedContact entities.Contact
			err := json.NewDecoder(resp.Body).Decode(&retrievedContact)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedContact.ID).To(Equal(testContact.ID))
		})

		It("should not find a non-existent contact", func() {
			req, _ := http.NewRequest(http.MethodGet, "/contacts/nonexistent", nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusNotFound))
		})

		It("should update a contact", func() {
			updatedContactPayload := createdContact
			updatedContactPayload.Email = "mike@doe.com"
			payloadBytes, _ := json.Marshal(updatedContactPayload)

			req, _ := http.NewRequest(http.MethodPut, "/contacts", bytes.NewReader(payloadBytes))
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusOK))

			var updatedContact entities.Contact
			err := json.NewDecoder(resp.Body).Decode(&updatedContact)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedContact.Email).To(Equal(updatedContactPayload.Email))
		})

		It("should not update a non-existent contact", func() {
			nonExistentContact := createdContact
			nonExistentContact.ID = "nonexistent"
			payloadBytes, _ := json.Marshal(nonExistentContact)

			req, _ := http.NewRequest(http.MethodPut, "/contacts", bytes.NewReader(payloadBytes))
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusNotFound))
		})

		It("should list all contacts", func() {
			req, _ := http.NewRequest(http.MethodGet, "/contacts", nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusOK))

			var contacts []entities.Contact
			err := json.NewDecoder(resp.Body).Decode(&contacts)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(contacts)).To(BeNumerically(">", 0))
		})

		It("should delete a contact by ID", func() {
			req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/contacts/%s", testContact.ID), nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusNoContent))
		})

		It("should not find the deleted contact", func() {
			req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/contacts/%s", testContact.ID), nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusNotFound))
		})
	})
})
