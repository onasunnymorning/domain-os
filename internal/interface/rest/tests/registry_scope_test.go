package tests

import (
	"fmt"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Registry scope integration test (#415, ADR-0006)
//
// Story: two operators, one TLD each. A destructive request names its operator
// in X-Tenant-ID and reaches only that operator's TLDs. Someone else's object
// reads as not found — and is still there afterwards. A request that names no
// operator, from a principal without the platform permission, is refused.
var _ = Describe("RegistryScope", Ordered, func() {
	const (
		ownerRy  = "rsApiOwnerRy"
		otherRy  = "rsApiOtherRy"
		ownerTLD = "rsapiowned"
		otherTLD = "rsapiother"
		nndn     = "reserved.rsapiowned"
	)

	BeforeAll(func() {
		for ry, tld := range map[string]string{ownerRy: ownerTLD, otherRy: otherTLD} {
			resp := api.POST("/registry-operators", &commands.CreateRegistryOperatorCommand{
				RyID: ry, Name: ry, Email: "ops@" + tld + ".example",
			})
			Expect(resp.Code).To(Equal(http.StatusCreated))
			resp = api.POST("/tlds", &request.CreateTLDRequest{Name: tld, RyID: ry})
			Expect(resp.Code).To(Equal(http.StatusCreated))
		}
		resp := api.POST("/nndns", &request.CreateNNDNRequest{Name: nndn, Reason: "reserved"})
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	It("refuses a delete that names no operator", func() {
		resp := api.DELETE(fmt.Sprintf("/nndns/%s", nndn))
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
		Expect(resp.Body.String()).To(ContainSubstring("X-Tenant-ID"))
		Expect(api.GET(fmt.Sprintf("/nndns/%s", nndn)).Code).To(Equal(http.StatusOK))
	})

	It("treats another operator's NNDN as not found, and leaves it in place", func() {
		resp := api.DELETEAs(fmt.Sprintf("/nndns/%s", nndn), otherRy)
		Expect(resp.Code).To(Equal(http.StatusNotFound))
		Expect(api.GET(fmt.Sprintf("/nndns/%s", nndn)).Code).To(Equal(http.StatusOK))
	})

	It("treats another operator's TLD as not found, and leaves it in place", func() {
		resp := api.DELETEAs(fmt.Sprintf("/tlds/%s", ownerTLD), otherRy)
		Expect(resp.Code).To(Equal(http.StatusNotFound))
		Expect(api.GET(fmt.Sprintf("/tlds/%s", ownerTLD)).Code).To(Equal(http.StatusOK))
	})

	It("lets the owning operator delete its own NNDN", func() {
		resp := api.DELETEAs(fmt.Sprintf("/nndns/%s", nndn), ownerRy)
		Expect(resp.Code).To(Equal(http.StatusNoContent))
		Expect(api.GET(fmt.Sprintf("/nndns/%s", nndn)).Code).To(Equal(http.StatusNotFound))
	})

	It("stays idempotent for an NNDN that genuinely does not exist", func() {
		resp := api.DELETEAs(fmt.Sprintf("/nndns/%s", nndn), ownerRy)
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	AfterAll(func() {
		api.DELETEAs(fmt.Sprintf("/tlds/%s", ownerTLD), ownerRy)
		api.DELETEAs(fmt.Sprintf("/tlds/%s", otherTLD), otherRy)
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ownerRy))
		api.DELETE(fmt.Sprintf("/registry-operators/%s", otherRy))
	})
})
