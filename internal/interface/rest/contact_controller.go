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
	e.PUT("/contacts/:id", controller.UpdateContact)
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
		} else {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	// Get the Event from the context
	event := GetEventFromContext(ctx)
	// Set the event details.command
	event.Details.Command = req

	// Create the contact
	contact, err := ctrl.contactService.CreateContact(ctx, &req)
	if err != nil {
		event.Details.Error = err
		if errors.Is(err, entities.ErrInvalidContact) ||
			errors.Is(err, entities.ErrContactAlreadyExists) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	// Set the event details.after and objectID
	event.Details.After = contact
	event.ObjectID = contact.RoID.String()

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
// @Router /contacts/:id [put]
func (ctrl *ContactController) UpdateContact(ctx *gin.Context) {
	var req commands.UpdateContactCommand
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the Event from the context
	e := GetEventFromContext(ctx)
	// Set the event details.command
	e.Details.Command = req

	// Look up the contact
	c, err := ctrl.contactService.GetContactByID(ctx, ctx.Param("id"))
	if err != nil {
		e.Details.Error = err
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set the event details.before
	e.Details.Before = c

	// Make the changes
	c.Email = req.Email
	c.AuthInfo = req.AuthInfo
	c.ClID = req.ClID
	c.CrRr = req.CrRr
	c.UpRr = req.UpRr
	c.PostalInfo = req.PostalInfo
	c.Voice = req.Voice
	c.Fax = req.Fax
	c.Status = req.Status
	c.Disclose = req.Disclose

	c.SetOKStatusIfNeeded()
	c.UnSetOKStatusIfNeeded()

	// Validate the changes
	_, err = c.IsValid()
	if err != nil {
		e.Details.Error = err
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contact, err := ctrl.contactService.UpdateContact(ctx, c)
	if err != nil {
		e.Details.Error = err
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	e.Details.After = contact

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
	e := newEventFromContext(ctx)

	err := ctrl.contactService.DeleteContactByID(ctx, id)
	if err != nil {
		if errors.Is(err, entities.ErrContactNotFound) {
			e.Details.Result = entities.EventResultSuccess
			ctx.JSON(http.StatusNoContent, nil)
		} else {
			e.Details.Result = entities.EventResultFailure
			e.Details.Error = err
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		logMessage(ctx, e)
		return
	}

	ctx.JSON(http.StatusNoContent, nil)
	e.Details.Before = id
	e.Details.Result = entities.EventResultSuccess
	logMessage(ctx, e)
}

// ListContacts godoc
// @Summary List contacts
// @Description List contacts
// @Tags Contacts
// @Produce json
// @Param pageSize query int false "Page Size"
// @Param cursor query string false "Cursor"
// @Success 200 {array} response.ListItemResult
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
