package tests

import (
	"fmt"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContactController", Ordered, func() {
	registrarClid := "contTestRar"

	testAddress, _ := entities.NewAddress("El Cuyo", "MX")
	testPostalInfo, _ := entities.NewContactPostalInfo("int", "my pinfo", testAddress)

	testContact := &commands.CreateContactCommand{
		ID:       "contactID101",
		RoID:     "12345_CONT-APEX",
		Email:    "jon@doe.com",
		AuthInfo: "str0NGP@ZZw0rd",
		ClID:     registrarClid,
		PostalInfo: [2]*entities.ContactPostalInfo{
			testPostalInfo,
		},
	}

	var createdContact entities.Contact

	// Create prerequisite registrar
	BeforeAll(func() {
		registrarPayload := testRegistrar(registrarClid, "Registrar for Contact Tests", 10002)
		resp := api.POST("/registrars", registrarPayload)
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	It("should successfully create a contact", func() {
		resp := api.POST("/contacts", testContact)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		err := DecodeJSON(resp, &createdContact)
		Expect(err).NotTo(HaveOccurred())
		Expect(createdContact.ID.String()).To(Equal(testContact.ID))
		Expect(createdContact.RoID.String()).To(Equal(testContact.RoID))
		Expect(createdContact.Email).To(Equal(testContact.Email))
		Expect(createdContact.AuthInfo.String()).To(Equal(testContact.AuthInfo))
	})

	It("should not create a contact with an existing ID", func() {
		resp := api.POST("/contacts", testContact)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should not create a contact with an invalid email", func() {
		invalidContact := &commands.CreateContactCommand{
			ID:       "contactID102",
			RoID:     "12345_CONT-APEX",
			Email:    "invalid-email",
			AuthInfo: "str0NGP@ZZw0rd",
		}
		resp := api.POST("/contacts", invalidContact)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should retrieve a contact by ID", func() {
		resp := api.GET(fmt.Sprintf("/contacts/%s", testContact.ID))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var retrievedContact entities.Contact
		err := DecodeJSON(resp, &retrievedContact)
		Expect(err).NotTo(HaveOccurred())
		Expect(retrievedContact.ID.String()).To(Equal(testContact.ID))
	})

	It("should not find a non-existent contact", func() {
		resp := api.GET("/contacts/nonexistent")
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	It("should update a contact", func() {
		updatedContactPayload := createdContact
		updatedContactPayload.Email = "mike@doe.com"

		resp := api.PUT(fmt.Sprintf("/contacts/%s", createdContact.ID.String()), updatedContactPayload)
		Expect(resp.Code).To(Equal(http.StatusOK))

		var updatedContact entities.Contact
		err := DecodeJSON(resp, &updatedContact)
		Expect(err).NotTo(HaveOccurred())
		Expect(updatedContact.Email).To(Equal(updatedContactPayload.Email))
	})

	It("should delete a contact by ID", func() {
		resp := api.DELETE(fmt.Sprintf("/contacts/%s", testContact.ID))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should not find the deleted contact", func() {
		resp := api.GET(fmt.Sprintf("/contacts/%s", testContact.ID))
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})
})
