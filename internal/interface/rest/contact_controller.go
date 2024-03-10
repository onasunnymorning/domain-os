package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

type ContactController struct {
	contactService interfaces.ContactService
}

func NewContactController(e *gin.Engine, contactService interfaces.ContactService) *ContactController {
	controller := &ContactController{
		contactService: contactService,
	}

	e.GET("/contacts", controller.ListContacts)
	e.GET("/contacts/:id", controller.GetContactByID)
	e.POST("/contacts", controller.CreateContact)
	e.PUT("/contacts", controller.UpdateContact)
	e.DELETE("/contacts/:id", controller.DeleteContactByID)

	return controller
}

// GetContactByID godoc
// @Summary Get a contact by ID
// @Description Get a contact by ID
// @Tags Contacts
// @Produce json
// @Param id path string true "Contact ID"
// @Success 200 {object} entities.Contact
// @Failure 404
// @Failure 500
// @Router /contacts/{id} [get]
func (ctrl *ContactController) GetContactByID(ctx *gin.Context) {
	id := ctx.Param("id")

	contact, err := ctrl.contactService.GetContactByID(ctx, id)
	if err != nil {
		if errors.Is(err, entities.ErrContactNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, contact)
}

// CreateContact godoc
// @Summary Create a contact
// @Description Create a contact
// @Tags Contacts
// @Accept json
// @Produce json
// @Param contact body commands.CreateContactCommand true "Contact"
// @Success 201 {object} entities.Contact
// @Failure 400
// @Failure 500
// @Router /contacts [post]
func (ctrl *ContactController) CreateContact(ctx *gin.Context) {
	var req commands.CreateContactCommand
	if err := ctx.ShouldBindJSON(&req); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contact, err := ctrl.contactService.CreateContact(ctx, &req)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidContact) ||
			errors.Is(err, entities.ErrContactAlreadyExists) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, contact)
}

// UpdateContact godoc
// @Summary Update a contact
// @Description Update a contact
// @Tags Contacts
// @Accept json
// @Produce json
// @Param contact body entities.Contact true "Contact"
// @Success 200 {object} entities.Contact
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /contacts [put]
func (ctrl *ContactController) UpdateContact(ctx *gin.Context) {
	var req entities.Contact
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := req.IsValid()
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contact, err := ctrl.contactService.UpdateContact(ctx, &req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, contact)
}

// DeleteContactByID godoc
// @Summary Delete a contact by ID
// @Description Delete a contact by ID
// @Tags Contacts
// @Produce json
// @Param id path string true "Contact ID"
// @Success 204
// @Failure 404
// @Failure 500
// @Router /contacts/{id} [delete]
func (ctrl *ContactController) DeleteContactByID(ctx *gin.Context) {
	id := ctx.Param("id")

	err := ctrl.contactService.DeleteContactByID(ctx, id)
	if err != nil {
		if errors.Is(err, entities.ErrContactNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusNoContent, nil)
}

// ListContacts godoc
// @Summary List contacts
// @Description List contacts
// @Tags Contacts
// @Produce json
// @Param pageSize query int false "Page Size"
// @Param cursor query string false "Cursor"
// @Success 200 {array} entities.Contact
// @Failure 400
// @Failure 500
// @Router /contacts [get]
func (ctrl *ContactController) ListContacts(ctx *gin.Context) {
	var err error
	// Prepare the response
	response := response.ListItemResult{}
	// Get the pagesize from the query string
	pageSize, err := GetPageSize(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Get the cursor from the query string
	pageCursor, err := GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Get the contacts from the service
	contacts, err := ctrl.contactService.ListContacts(ctx, pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	response.Data = contacts
	if len(contacts) > 0 {
		response.SetMeta(ctx, contacts[len(contacts)-1].RoID.String(), len(contacts), pageSize)
	}

	// Return the response
	ctx.JSON(200, response)

}
