package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
)

// SyncController is the controller for Sync endpoints
type SyncController struct {
	syncService interfaces.SyncService
}

// SyncResult is the successful result of a sync endpoint
type SyncResult struct {
	Message string `json:"Message"`
}

// NewSyncController creates a new SyncController and registers the endpoints
func NewSyncController(e *gin.Engine, syncService interfaces.SyncService) *SyncController {
	controller := &SyncController{
		syncService: syncService,
	}

	e.PUT("/sync/icann-spec5", controller.SyncSpec5)
	e.PUT("/sync/iana-registrars", controller.SyncRegistrars)
	e.PUT("/sync/fx/:currency", controller.SyncFX)

	return controller
}

// SyncSpec5 godoc
// @Summary Sync Spec5 labels from ICANN to the database
// @Description Reads in the spec5 labels from ICANN XML repository (https://www.icann.org/sites/default/files/packages/reserved-names/ReservedNames.xml) and refreshes the database.
// @Description This will replace all spec5Labels in the database. Its recommended to first backup the current spec5Labels.
// @Description Use this endpoint when there is an update to the spec5 policy by ICANN. See this webpage for reference (https://www.icann.org/reserved-names-en).
// @Description Expect this endpoint to be slow, as it downloads and processes the XML file from another server and then updates the database.
// @Tags Sync
// @Produce json
// @Success 200 {object} SyncResult
// @Failure 500
// @Router /sync/icann-spec5 [put]
func (ctrl *SyncController) SyncSpec5(ctx *gin.Context) {
	err := ctrl.syncService.RefreshSpec5Labels()
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	result := &SyncResult{Message: "Successfully synced spec5 labels"}

	ctx.JSON(200, result)
}

// SyncRegistrars godoc
// @Summary Sync Registrars from IANA to the database
// @Description Reads in the registrars from IANA XML repository (https://www.iana.org/assignments/registrar-ids/registrar-ids.xhtml) and refreshes the database.
// @Description This will replace all IANARegistrars in the database. Its recommended to first backup the current IANARegistrars.
// @Description Use this endpoint when there is an update to the IANA registrar list or you are notified by ICANN of a termination of a registrar.
// @Description Expect this endpoint to be slow, as it downloads and processes the XML file from another server and then updates the database.
// @Tags Sync
// @Produce json
// @Success 200 {object} SyncResult
// @Failure 500
// @Router /sync/iana-registrars [put]
func (ctrl *SyncController) SyncRegistrars(ctx *gin.Context) {
	err := ctrl.syncService.RefreshIANARegistrars()
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	result := &SyncResult{Message: "Successfully synced IANA registrars"}

	ctx.JSON(200, result)
}

// SyncFX godoc
// @Summary Sync FX from OpenFX to the database
// @Description Reads in the exchange rates from OpenFX API (https://openexchangerates.org/) and refreshes the database.
// @Description This will replace all FXRates in the database.
// @Description Use this endpoint when there is an update to the exchange rates by OpenFX.
// @Description Expect this endpoint to be slow, as it downloads and processes the JSON file from another server and then updates the database.
// @Tags Sync
// @Produce json
// @Param currency path string true "The base currency to sync"
// @Success 200 {object} SyncResult
// @Failure 500
// @Router /sync/fx/:currency [put]
func (ctrl *SyncController) SyncFX(ctx *gin.Context) {
	baseCurrency := ctx.Param("currency")
	err := ctrl.syncService.RefreshFXRates(baseCurrency)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	result := &SyncResult{Message: "Successfully synced FX rates"}

	ctx.JSON(200, result)
}
