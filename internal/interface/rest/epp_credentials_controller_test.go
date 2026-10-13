package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/require"
)

type fakeEPPCredentialService struct {
	access  *entities.EPPRegistrarAccess
	retired []entities.EPPCredentialRef
	err     error
}

func (f *fakeEPPCredentialService) GetAccess(context.Context, string) (*entities.EPPRegistrarAccess, error) {
	return f.access, f.err
}

func (f *fakeEPPCredentialService) CreateLogin(_ context.Context, _ string, cmd commands.CreateEPPLoginCommand) (*entities.EPPLogin, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &entities.EPPLogin{LoginID: entities.ClIDType(cmd.LoginID), Role: entities.EPPLoginRole(cmd.Role), Enabled: true}, nil
}

func (f *fakeEPPCredentialService) UpdateLogin(context.Context, string, string, commands.UpdateEPPLoginCommand) (*entities.EPPLogin, error) {
	return nil, f.err
}

func (f *fakeEPPCredentialService) AddPassword(_ context.Context, _, _ string, cmd commands.AddEPPPasswordCommand) (*entities.EPPPassword, string, error) {
	if f.err != nil {
		return nil, "", f.err
	}
	p := &entities.EPPPassword{EPPCredentialLifecycle: entities.EPPCredentialLifecycle{ID: uuid.New()}, Hash: "hash:secret"}
	if cmd.Password == "" {
		return p, "Generated-16char", nil
	}
	return p, "", nil
}

func (f *fakeEPPCredentialService) AddCertificatePin(context.Context, string, commands.AddEPPCertificatePinCommand) (*entities.EPPCertificatePin, error) {
	return nil, f.err
}

func (f *fakeEPPCredentialService) AddAllowedNetwork(context.Context, string, commands.AddEPPAllowedNetworkCommand) (*entities.EPPAllowedNetwork, error) {
	return nil, f.err
}

func (f *fakeEPPCredentialService) RetireCredential(_ context.Context, _ string, ref entities.EPPCredentialRef, _ commands.RetireEPPCredentialCommand) error {
	f.retired = append(f.retired, ref)
	return f.err
}

// eppCredentialRouter serves the controller, granting scopes to every request.
func eppCredentialRouter(svc *fakeEPPCredentialService, scopes ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	grant := func(c *gin.Context) {
		c.Request = c.Request.WithContext(appcontext.WithAuthScopes(c.Request.Context(), scopes))
		c.Next()
	}
	NewEPPCredentialsController(r, svc, grant)
	return r
}

func serveEPPCredential(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestEPPCredentialsRequirePermission(t *testing.T) {
	r := eppCredentialRouter(&fakeEPPCredentialService{}, ScopeEscrowKeysAdmin)
	w := serveEPPCredential(r, http.MethodGet, "/registrars/rar-001/epp", "")
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), ScopeEPPCredentialsAdmin)
}

func TestEPPCredentialsAccessViewShowsStateNotHashes(t *testing.T) {
	now := time.Now().UTC()
	retireAt := now.Add(time.Hour)
	access := &entities.EPPRegistrarAccess{
		ClID: "rar-001",
		Logins: []*entities.EPPLogin{{LoginID: "rar-001", Role: entities.EPPLoginRoleTransform, Enabled: true, Passwords: []*entities.EPPPassword{
			{EPPCredentialLifecycle: entities.EPPCredentialLifecycle{ID: uuid.New(), CreatedAt: now, RetireAt: &retireAt, RetireKind: entities.EPPRetireNormal}, Hash: "$argon2id$secret"},
		}}},
	}
	r := eppCredentialRouter(&fakeEPPCredentialService{access: access}, ScopeEPPCredentialsAdmin)
	w := serveEPPCredential(r, http.MethodGet, "/registrars/rar-001/epp", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "argon2id")

	var got struct {
		Logins []struct {
			LoginID   string `json:"login_id"`
			Passwords []struct {
				ID    string `json:"id"`
				State string `json:"state"`
			} `json:"passwords"`
		} `json:"logins"`
		CertificatePins []any `json:"certificate_pins"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "retiring", got.Logins[0].Passwords[0].State)
	require.NotNil(t, got.CertificatePins, "empty lists are [], not null")
}

func TestEPPCredentialsGeneratedPasswordIsShownOnce(t *testing.T) {
	r := eppCredentialRouter(&fakeEPPCredentialService{}, ScopeEPPCredentialsAdmin)

	w := serveEPPCredential(r, http.MethodPost, "/registrars/rar-001/epp/logins/rar-001/passwords", "")
	require.Equal(t, http.StatusCreated, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Contains(t, w.Body.String(), `"password":"Generated-16char"`)
	require.NotContains(t, w.Body.String(), "hash:")

	w = serveEPPCredential(r, http.MethodPost, "/registrars/rar-001/epp/logins/rar-001/passwords", `{"password":"chosen-pw-1"}`)
	require.Equal(t, http.StatusCreated, w.Code)
	require.NotContains(t, w.Body.String(), "chosen-pw-1")
	require.NotContains(t, w.Body.String(), `"password"`)
}

func TestEPPCredentialsRetireRoutesNameTheKind(t *testing.T) {
	svc := &fakeEPPCredentialService{}
	r := eppCredentialRouter(svc, ScopeEPPCredentialsAdmin)
	id := uuid.New()

	for path, want := range map[string]entities.EPPCredentialRef{
		"/registrars/rar-001/epp/logins/rar-001/passwords/" + id.String() + "/retire": {Kind: entities.EPPCredentialPassword, LoginID: "rar-001", ID: id},
		"/registrars/rar-001/epp/certificate-pins/" + id.String() + "/retire":         {Kind: entities.EPPCredentialCertificatePin, ID: id},
		"/registrars/rar-001/epp/allowed-networks/" + id.String() + "/retire":         {Kind: entities.EPPCredentialAllowedNetwork, ID: id},
	} {
		svc.retired = nil
		w := serveEPPCredential(r, http.MethodPost, path, `{"kind":"compromise"}`)
		require.Equal(t, http.StatusOK, w.Code, path)
		require.Equal(t, []entities.EPPCredentialRef{want}, svc.retired, path)
	}

	w := serveEPPCredential(r, http.MethodPost, "/registrars/rar-001/epp/certificate-pins/not-a-uuid/retire", "")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEPPCredentialsErrorMapping(t *testing.T) {
	for err, code := range map[error]int{
		entities.ErrRegistrarNotFound:        http.StatusNotFound,
		entities.ErrEPPCredentialNotFound:    http.StatusNotFound,
		entities.ErrEPPLoginExists:           http.StatusConflict,
		entities.ErrEPPCredentialLimit:       http.StatusConflict,
		entities.ErrEPPCredentialRetired:     http.StatusConflict,
		entities.ErrInvalidEPPPassword:       http.StatusBadRequest,
		entities.ErrInvalidEPPCertificate:    http.StatusBadRequest,
		entities.ErrInvalidEPPAllowedNetwork: http.StatusBadRequest,
		context.DeadlineExceeded:             http.StatusInternalServerError,
	} {
		r := eppCredentialRouter(&fakeEPPCredentialService{err: err}, ScopeEPPCredentialsAdmin)
		w := serveEPPCredential(r, http.MethodPost, "/registrars/rar-001/epp/logins", `{"login_id":"rar-001"}`)
		require.Equal(t, code, w.Code, err.Error())
	}
}
