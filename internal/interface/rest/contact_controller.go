package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

type ContactController struct {
	contactService interfaces.ContactService
}

func NewContactController(e *gin.Engine, contactService interfaces.ContactService, handler gin.HandlerFunc) *ContactController {
	controller := &ContactController{
		contactService: contactService,
	}

	contactGroup := e.Group("/contacts", handler)
	{
		contactGroup.GET("", controller.ListContacts)
		contactGroup.GET("count", controller.CountContacts)
		contactGroup.GET(":id", controller.GetContactByID)
		contactGroup.POST("", controller.CreateContact)
		contactGroup.POST("/bulk", controller.BulkCreateContacts)
		contactGroup.PUT(":id", controller.UpdateContact)
		contactGroup.DELETE(":id", controller.DeleteContactByID)
	}
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

	contact, err := ctrl.contactService.GetContactByID(ctx.Request.Context(), id)
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

	// Create the contact
	contact, err := ctrl.contactService.CreateContact(ctx.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidContact) ||
			errors.Is(err, entities.ErrContactAlreadyExists) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	ctx.JSON(http.StatusCreated, contact)
}

// BulkCreateContacts godoc
// @Summary Bulk create contacts - for import only
// @Description Bulk create contacts, useful when importing data, not for normal domain operations.
// @Description If any of the contacts is invalid, it returns an error and does not save any of the contacts
// @Tags Contacts
// @Accept json
// @Produce json
// @Param contacts body []commands.CreateContactCommand true "Contacts"
// @Success 201
// @Failure 400
// @Failure 500
// @Router /contacts/bulk [post]
func (ctrl *ContactController) BulkCreateContacts(ctx *gin.Context) {
	var req []*commands.CreateContactCommand
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create the contacts
	err := ctrl.contactService.BulkCreate(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidContact) ||
			errors.Is(err, entities.ErrContactAlreadyExists) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	ctx.JSON(http.StatusCreated, nil)
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
// @Router /contacts/{id} [put]
func (ctrl *ContactController) UpdateContact(ctx *gin.Context) {
	var req commands.UpdateContactCommand
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Look up the contact
	c, err := ctrl.contactService.GetContactByID(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contact, err := ctrl.contactService.UpdateContact(ctx.Request.Context(), c)
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

	err := ctrl.contactService.DeleteContactByID(ctx.Request.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrContactNotFound) {
			ctx.JSON(http.StatusNoContent, nil)
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
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
// @Success 200 {array} response.ListItemResult
// @Failure 400
// @Failure 500
// @Router /contacts [get]
func (ctrl *ContactController) ListContacts(ctx *gin.Context) {
	query := queries.ListItemsQuery{}
	var err error
	// Prepare the response
	response := response.ListItemResult{}
	// Get the pagesize from the query string
	query.PageSize, err = GetPageSize(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Get the cursor from the query string
	query.PageCursor, err = GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Set the Filters
	filter := queries.ListContactsFilter{}
	filter.RoidGreaterThan = ctx.Query("roid_greater_than")
	filter.RoidLessThan = ctx.Query("roid_less_than")
	filter.IdLike = ctx.Query("id_like")
	filter.EmailLike = ctx.Query("email_like")
	filter.ClidEquals = ctx.Query("clid_equals")
	query.Filter = filter

	// Get the contacts from the service
	contacts, cursor, err := ctrl.contactService.ListContacts(ctx.Request.Context(), query)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Set the response metadata
	response.Data = contacts
	response.SetMeta(ctx, cursor, len(contacts), query.PageSize, query.Filter)

	// Return the response
	ctx.JSON(200, response)

}

// CountContacts godoc
// @Summary Count contacts
// @Description Count contacts
// @Tags Contacts
// @Produce json
// @Param id_like query string false "ID Like"
// @Param email_like query string false "Email Like"
// @Param clid_equals query string false "Clid Equals"
// @Success 200 {object} response.CountResult
// @Failure 400
// @Failure 500
// @Router /contacts/count [get]
func (ctrl *ContactController) CountContacts(ctx *gin.Context) {
	filter := queries.ListContactsFilter{}
	filter.IdLike = ctx.Query("id_like")
	filter.EmailLike = ctx.Query("email_like")
	filter.ClidEquals = ctx.Query("clid_equals")

	count, err := ctrl.contactService.Count(ctx.Request.Context(), filter)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response.CountResult{
		Count: count,
	})
}
