package tests

import (
	"fmt"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Registrar Extended Integration Test
//
// Extends the basic registrar CRUD tests with:
//   - Status management (PUT /registrars/:clid/status/:status)
//   - IANA status management (PUT /registrars/:clid/iana_status/:status)
//   - Update registrar (PUT /registrars/:clid)
//   - Search/filter (GET /registrars?name_like=...)
//   - Count with filter
//   - Pagination
var _ = Describe("RegistrarExtended", Ordered, func() {
	const (
		rar1ClID = "extRar1"
		rar1GurID = 10012
	)

	BeforeAll(func() {
		// Create registrar
		rar1Payload := testRegistrar(rar1ClID, "Extended Test Registrar One", rar1GurID)
		resp := api.POST("/registrars", rar1Payload)
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	// ================================================================== //
	//  Status Management                                                  //
	// ================================================================== //

	It("should set registrar status to ok", func() {
		resp := api.PUT(fmt.Sprintf("/registrars/%s/status/ok", rar1ClID), nil)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusOK),
			Equal(http.StatusNoContent),
		))
	})

	It("should verify registrar status is ok", func() {
		resp := api.GET(fmt.Sprintf("/registrars/%s", rar1ClID))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var rar entities.Registrar
		Expect(DecodeJSON(resp, &rar)).To(Succeed())
		Expect(rar.Status).To(Equal(entities.RegistrarStatusOK))
	})

	It("should set IANA status to Accredited", func() {
		resp := api.PUT(fmt.Sprintf("/registrars/%s/iana_status/Accredited", rar1ClID), nil)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusOK),
			Equal(http.StatusNoContent),
		))
	})

	It("should verify IANA status is Accredited", func() {
		resp := api.GET(fmt.Sprintf("/registrars/%s", rar1ClID))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var rar entities.Registrar
		Expect(DecodeJSON(resp, &rar)).To(Succeed())
		Expect(string(rar.IANAStatus)).To(Equal("Accredited"))
	})

	// ================================================================== //
	//  Update Registrar                                                   //
	// ================================================================== //

	It("should update the registrar name and URL", func() {
		// First GET the current registrar to have the full entity
		resp := api.GET(fmt.Sprintf("/registrars/%s", rar1ClID))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var rar entities.Registrar
		Expect(DecodeJSON(resp, &rar)).To(Succeed())

		// Modify fields
		rar.Name = "Updated Ext Registrar One"
		rar.NickName = "UpdatedExt1"

		resp = api.PUT(fmt.Sprintf("/registrars/%s", rar1ClID), rar)
		Expect(resp.Code).To(Equal(http.StatusOK))

		var updated entities.Registrar
		Expect(DecodeJSON(resp, &updated)).To(Succeed())
		Expect(updated.Name).To(Equal("Updated Ext Registrar One"))
		Expect(updated.NickName).To(Equal("UpdatedExt1"))
	})

	// ================================================================== //
	//  Search & List                                                      //
	// ================================================================== //

	It("should search registrars by nick_name_like", func() {
		resp := api.GET("/registrars?nick_name_like=UpdatedExt")
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListRegistrarsFilter{}
		res.Meta.Filter = &filter
		Expect(DecodeJSON(resp, &res)).To(Succeed())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(BeNumerically(">=", 1))
	})

	It("should count registrars", func() {
		resp := api.GET("/registrars/count")
		Expect(resp.Code).To(Equal(http.StatusOK))

		var countResult response.CountResult
		Expect(DecodeJSON(resp, &countResult)).To(Succeed())
		Expect(countResult.Count).To(BeNumerically(">", 0))
	})

	It("should list registrars with pagination (pagesize=1)", func() {
		resp := api.GET("/registrars?pagesize=1")
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListRegistrarsFilter{}
		res.Meta.Filter = &filter
		Expect(DecodeJSON(resp, &res)).To(Succeed())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(BeNumerically("<=", 1))
	})

	// ------------------------------------------------------------------ //
	//  AfterAll: best-effort cleanup (no assertions)                      //
	// ------------------------------------------------------------------ //
	AfterAll(func() {
		api.DELETE(fmt.Sprintf("/registrars/%s", rar1ClID))
	})
})
