package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/buildinfo"
)

// PingController is a controller for the ping endpoint
type PingController struct {
}

// NewPingController creates a new ping controller
func NewPingController(e *gin.Engine) *PingController {
	controller := &PingController{}

	e.GET("/ping", controller.Ping)

	return controller
}

// Ping is the handler for the ping endpoint
func (ctrl *PingController) Ping(ctx *gin.Context) {
	ctx.JSON(200, gin.H{
		"message":    "pong",
		"version":    buildinfo.Version,
		"git_sha":    buildinfo.GitSHA,
		"build_date": buildinfo.BuildDate,
	})
}
