package tests

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("EscrowSanitizationController", Ordered, func() {
	const (
		ryID    = "sanop"
		otherRy = "sanother"
		tldName = "santest"
	)

	BeforeAll(func() {
		resp := api.POST("/registry-operators", &commands.CreateRegistryOperatorCommand{RyID: ryID, Name: "Sanitize Test Operator", Email: "san@example.com"})
		Expect(resp.Code).To(Equal(http.StatusCreated))
		resp = api.POST("/registry-operators", &commands.CreateRegistryOperatorCommand{RyID: otherRy, Name: "Other Sanitize Operator", Email: "sanother@example.com"})
		Expect(resp.Code).To(Equal(http.StatusCreated))
		resp = api.POST("/tlds", &request.CreateTLDRequest{Name: tldName, RyID: ryID})
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	AfterAll(func() {
		api.DELETE("/tlds/" + tldName)
		api.DELETE("/registry-operators/" + ryID)
		api.DELETE("/registry-operators/" + otherRy)
	})

	It("requires the operator scope header", func() {
		resp := scoped(http.MethodPost, "/escrow/sanitizations", "", map[string]string{"sourceValidationRunId": uuid.New().String()})
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
		resp = scoped(http.MethodGet, "/escrow/sanitizations", "", nil)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("rejects a launch without a usable source id", func() {
		resp := scoped(http.MethodPost, "/escrow/sanitizations", ryID, map[string]string{})
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
		resp = scoped(http.MethodPost, "/escrow/sanitizations", ryID, map[string]string{"sourceValidationRunId": "not-a-uuid"})
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("hides a validation run the caller does not own", func() {
		// A source that does not exist for this tenant is a 404 rather than a
		// 403, so run ids cannot be probed across tenants.
		resp := scoped(http.MethodPost, "/escrow/sanitizations", otherRy, map[string]string{"sourceValidationRunId": uuid.New().String()})
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	It("lists nothing for a fresh tenant and 404s unknown runs", func() {
		resp := scoped(http.MethodGet, "/escrow/sanitizations?tld="+tldName, ryID, nil)
		Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
		var out struct {
			Items []rest.EscrowSanitizationRunResponse `json:"items"`
			Count int                                  `json:"count"`
		}
		Expect(json.Unmarshal(resp.Body.Bytes(), &out)).To(Succeed())
		Expect(out.Count).To(Equal(0))

		resp = scoped(http.MethodGet, "/escrow/sanitizations/"+uuid.New().String(), ryID, nil)
		Expect(resp.Code).To(Equal(http.StatusNotFound))
		resp = scoped(http.MethodGet, "/escrow/sanitizations/not-a-uuid", ryID, nil)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("rejects an invalid filter rather than ignoring it", func() {
		resp := scoped(http.MethodGet, "/escrow/sanitizations?tld=not%20a%20tld", ryID, nil)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
		resp = scoped(http.MethodGet, "/escrow/sanitizations?sourceValidationRunId=nope", ryID, nil)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})
})
