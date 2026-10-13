package rest

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPCredentialsController manages registrars' EPP logins, passwords,
// certificate pins and allowed networks (ADR 0014). Every endpoint needs the
// epp:credentials:admin permission. Changes reach running EPP servers within
// their credential refresh interval (docs/epp/POLICY_REFERENCE.md).
type EPPCredentialsController struct {
	svc interfaces.EPPCredentialService
	now func() time.Time
}

// NewEPPCredentialsController registers the routes under
// /registrars/:clid/epp.
func NewEPPCredentialsController(e *gin.Engine, svc interfaces.EPPCredentialService, handler gin.HandlerFunc) *EPPCredentialsController {
	ctrl := &EPPCredentialsController{svc: svc, now: time.Now}
	g := e.Group("/registrars/:clid/epp", handler, ctrl.requirePermission)
	{
		g.GET("", ctrl.GetAccess)
		g.POST("/logins", ctrl.CreateLogin)
		g.PATCH("/logins/:login", ctrl.UpdateLogin)
		g.POST("/logins/:login/passwords", ctrl.AddPassword)
		g.POST("/logins/:login/passwords/:id/retire", ctrl.retire(entities.EPPCredentialPassword))
		g.POST("/certificate-pins", ctrl.AddCertificatePin)
		g.POST("/certificate-pins/:id/retire", ctrl.retire(entities.EPPCredentialCertificatePin))
		g.POST("/allowed-networks", ctrl.AddAllowedNetwork)
		g.POST("/allowed-networks/:id/retire", ctrl.retire(entities.EPPCredentialAllowedNetwork))
	}
	return ctrl
}

func (ctrl *EPPCredentialsController) requirePermission(ctx *gin.Context) {
	if !requireAuthScope(ctx, ScopeEPPCredentialsAdmin) {
		return
	}
	ctx.Next()
}

// EPPCredentialState is a credential as the API shows it: its stored fields
// plus its state now.
type eppCredentialView struct {
	State entities.EPPCredentialState `json:"state"`
}

type eppPasswordView struct {
	*entities.EPPPassword
	eppCredentialView
}

type eppLoginView struct {
	*entities.EPPLogin
	Passwords []eppPasswordView `json:"passwords"`
}

type eppPinView struct {
	*entities.EPPCertificatePin
	eppCredentialView
}

type eppNetworkView struct {
	*entities.EPPAllowedNetwork
	eppCredentialView
}

type eppAccessView struct {
	ClID            entities.ClIDType `json:"clid"`
	Logins          []eppLoginView    `json:"logins"`
	CertificatePins []eppPinView      `json:"certificate_pins"`
	AllowedNetworks []eppNetworkView  `json:"allowed_networks"`
}

func (ctrl *EPPCredentialsController) loginView(l *entities.EPPLogin, now time.Time) eppLoginView {
	v := eppLoginView{EPPLogin: l, Passwords: []eppPasswordView{}}
	for _, p := range l.Passwords {
		v.Passwords = append(v.Passwords, eppPasswordView{p, eppCredentialView{p.State(now)}})
	}
	return v
}

func (ctrl *EPPCredentialsController) accessView(a *entities.EPPRegistrarAccess) eppAccessView {
	now := ctrl.now()
	v := eppAccessView{ClID: a.ClID, Logins: []eppLoginView{}, CertificatePins: []eppPinView{}, AllowedNetworks: []eppNetworkView{}}
	for _, l := range a.Logins {
		v.Logins = append(v.Logins, ctrl.loginView(l, now))
	}
	for _, p := range a.CertificatePins {
		v.CertificatePins = append(v.CertificatePins, eppPinView{p, eppCredentialView{p.State(now)}})
	}
	for _, n := range a.AllowedNetworks {
		v.AllowedNetworks = append(v.AllowedNetworks, eppNetworkView{n, eppCredentialView{n.State(now)}})
	}
	return v
}

// GetAccess godoc
// @Summary Get a registrar's EPP credentials
// @Description Logins with their passwords (never the hashes), certificate pins and allowed networks, each with its state now
// @Tags EPP credentials
// @Produce json
// @Param clid path string true "Registrar client ID"
// @Success 200
// @Failure 403
// @Failure 404
// @Router /registrars/{clid}/epp [get]
func (ctrl *EPPCredentialsController) GetAccess(ctx *gin.Context) {
	a, err := ctrl.svc.GetAccess(ctx.Request.Context(), ctx.Param("clid"))
	if err != nil {
		eppCredentialError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, ctrl.accessView(a))
}

// CreateLogin godoc
// @Summary Add an EPP login to a registrar
// @Description The login ID is the clID the registrar's system sends in <login>. It must be unique and may not be another registrar's client ID. Role is transform (default) or readonly
// @Tags EPP credentials
// @Accept json
// @Produce json
// @Param clid path string true "Registrar client ID"
// @Param body body commands.CreateEPPLoginCommand true "Login"
// @Success 201
// @Failure 400
// @Failure 404
// @Failure 409
// @Router /registrars/{clid}/epp/logins [post]
func (ctrl *EPPCredentialsController) CreateLogin(ctx *gin.Context) {
	var cmd commands.CreateEPPLoginCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	l, err := ctrl.svc.CreateLogin(ctx.Request.Context(), ctx.Param("clid"), cmd)
	if err != nil {
		eppCredentialError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, ctrl.loginView(l, ctrl.now()))
}

// UpdateLogin godoc
// @Summary Change an EPP login
// @Description Set role, enabled or session_cap (0 is the policy default). Disabling a login refuses new logins; open sessions run until they end
// @Tags EPP credentials
// @Accept json
// @Produce json
// @Param clid path string true "Registrar client ID"
// @Param login path string true "Login ID"
// @Param body body commands.UpdateEPPLoginCommand true "Changes"
// @Success 200
// @Failure 400
// @Failure 404
// @Router /registrars/{clid}/epp/logins/{login} [patch]
func (ctrl *EPPCredentialsController) UpdateLogin(ctx *gin.Context) {
	var cmd commands.UpdateEPPLoginCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	l, err := ctrl.svc.UpdateLogin(ctx.Request.Context(), ctx.Param("clid"), ctx.Param("login"), cmd)
	if err != nil {
		eppCredentialError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, ctrl.loginView(l, ctrl.now()))
}

type eppAddPasswordResponse struct {
	eppPasswordView
	// Password is set only when the server generated it. It is shown once and
	// cannot be retrieved again.
	Password string `json:"password,omitempty"`
}

// AddPassword godoc
// @Summary Add a password to an EPP login
// @Description Without a body (or with an empty password) the server generates a 16-character password and returns it once. A login has at most two usable passwords
// @Tags EPP credentials
// @Accept json
// @Produce json
// @Param clid path string true "Registrar client ID"
// @Param login path string true "Login ID"
// @Param body body commands.AddEPPPasswordCommand false "Password"
// @Success 201
// @Failure 400
// @Failure 404
// @Failure 409
// @Router /registrars/{clid}/epp/logins/{login}/passwords [post]
func (ctrl *EPPCredentialsController) AddPassword(ctx *gin.Context) {
	var cmd commands.AddEPPPasswordCommand
	if ctx.Request.ContentLength != 0 {
		if err := ctx.ShouldBindJSON(&cmd); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	p, generated, err := ctrl.svc.AddPassword(ctx.Request.Context(), ctx.Param("clid"), ctx.Param("login"), cmd)
	if err != nil {
		eppCredentialError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusCreated, eppAddPasswordResponse{eppPasswordView{p, eppCredentialView{p.State(ctrl.now())}}, generated})
}

// AddCertificatePin godoc
// @Summary Pin a client certificate's key for a registrar
// @Description Pins the SHA-256 of the certificate's public key. A pin may be added before the certificate is valid; an expired certificate is refused. logins restricts the pin to those logins. At most three pins are usable at once
// @Tags EPP credentials
// @Accept json
// @Produce json
// @Param clid path string true "Registrar client ID"
// @Param body body commands.AddEPPCertificatePinCommand true "PEM certificate"
// @Success 201
// @Failure 400
// @Failure 404
// @Failure 409
// @Router /registrars/{clid}/epp/certificate-pins [post]
func (ctrl *EPPCredentialsController) AddCertificatePin(ctx *gin.Context) {
	var cmd commands.AddEPPCertificatePinCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := ctrl.svc.AddCertificatePin(ctx.Request.Context(), ctx.Param("clid"), cmd)
	if err != nil {
		eppCredentialError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, eppPinView{p, eppCredentialView{p.State(ctrl.now())}})
}

// AddAllowedNetwork godoc
// @Summary Allow a source network for a registrar's EPP connections
// @Description A CIDR (no broader than /24 for IPv4 or /48 for IPv6) or a single address. At most twenty are usable at once
// @Tags EPP credentials
// @Accept json
// @Produce json
// @Param clid path string true "Registrar client ID"
// @Param body body commands.AddEPPAllowedNetworkCommand true "Network"
// @Success 201
// @Failure 400
// @Failure 404
// @Failure 409
// @Router /registrars/{clid}/epp/allowed-networks [post]
func (ctrl *EPPCredentialsController) AddAllowedNetwork(ctx *gin.Context) {
	var cmd commands.AddEPPAllowedNetworkCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	n, err := ctrl.svc.AddAllowedNetwork(ctx.Request.Context(), ctx.Param("clid"), cmd)
	if err != nil {
		eppCredentialError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, eppNetworkView{n, eppCredentialView{n.State(ctrl.now())}})
}

// retire godoc
// @Summary Retire an EPP password, certificate pin or allowed network
// @Description kind "normal" (default) refuses new logins from "at" (default: in 7 days; a time not in the future means now) while open sessions run on. kind "compromise" takes effect at once and also ends every session that used the credential
// @Tags EPP credentials
// @Accept json
// @Produce json
// @Param clid path string true "Registrar client ID"
// @Param id path string true "Credential ID"
// @Param body body commands.RetireEPPCredentialCommand false "How and when"
// @Success 200
// @Failure 400
// @Failure 404
// @Failure 409
// @Router /registrars/{clid}/epp/certificate-pins/{id}/retire [post]
// @Router /registrars/{clid}/epp/allowed-networks/{id}/retire [post]
// @Router /registrars/{clid}/epp/logins/{login}/passwords/{id}/retire [post]
func (ctrl *EPPCredentialsController) retire(kind entities.EPPCredentialKind) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id, ok := pathUUID(ctx, "id")
		if !ok {
			return
		}
		var cmd commands.RetireEPPCredentialCommand
		if ctx.Request.ContentLength != 0 {
			if err := ctx.ShouldBindJSON(&cmd); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		ref := entities.EPPCredentialRef{Kind: kind, LoginID: ctx.Param("login"), ID: id}
		if err := ctrl.svc.RetireCredential(ctx.Request.Context(), ctx.Param("clid"), ref, cmd); err != nil {
			eppCredentialError(ctx, err)
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"id": id, "kind": kind, "retired": true})
	}
}

func eppCredentialError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, entities.ErrRegistrarNotFound), errors.Is(err, entities.ErrEPPLoginNotFound),
		errors.Is(err, entities.ErrEPPCredentialNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrEPPLoginExists), errors.Is(err, entities.ErrEPPCredentialRetired),
		errors.Is(err, entities.ErrEPPCredentialLimit), errors.Is(err, entities.ErrEPPCertificatePinExists),
		errors.Is(err, entities.ErrEPPAllowedNetworkExists):
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrInvalidEPPCredential), errors.Is(err, entities.ErrInvalidEPPPassword),
		errors.Is(err, entities.ErrInvalidEPPCertificate), errors.Is(err, entities.ErrInvalidEPPAllowedNetwork):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "the EPP credential request failed"})
	}
}
