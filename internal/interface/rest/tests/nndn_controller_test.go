package tests

import (
	"fmt"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// NNDN Controller Integration Test
//
// Story: manage Non-standard Domain Names through their full lifecycle
//
//	create → list → count → get → delete → verify deletion
var _ = Describe("NNDNController", Ordered, func() {
	const (
		ryID     = "nndnTestRyOp"
		tldName  = "nndntest"
		nndn1    = "reserved1.nndntest"
		nndn2    = "reserved2.nndntest"
	)

	// ------------------------------------------------------------------ //
	//  Setup: create prerequisite entities                                //
	// ------------------------------------------------------------------ //
	BeforeAll(func() {
		// 1. Registry Operator
		ryCmd := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID,
			Name:  "NNDN Test RyOp",
			Email: "admin@nndntest.com",
		}
		resp := api.POST("/registry-operators", ryCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 2. TLD
		tldReq := &request.CreateTLDRequest{
			Name: tldName,
			RyID: ryID,
		}
		resp = api.POST("/tlds", tldReq)
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	// ================================================================== //
	//  List — zero results initially                                      //
	// ================================================================== //

	It("should list NNDNs with zero results for the test TLD", func() {
		resp := api.GET(fmt.Sprintf("/nndns?tld_equals=%s", tldName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListNndnsFilter{}
		res.Meta.Filter = &filter
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		if res.Data != nil {
			itemSlice, ok := res.Data.([]interface{})
			if ok {
				Expect(itemSlice).To(BeEmpty())
			}
		}
	})

	// ================================================================== //
	//  Create — unhappy paths                                             //
	// ================================================================== //

	It("should fail to create NNDN with no body", func() {
		resp := api.POSTNoBody("/nndns")
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should fail to create NNDN with missing name", func() {
		cmd := map[string]interface{}{
			"Reason": "reserved",
		}
		resp := api.POST("/nndns", cmd)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should fail to create NNDN with invalid name", func() {
		cmd := &request.CreateNNDNRequest{
			Name:   "notavalidname",
			Reason: "reserved",
		}
		resp := api.POST("/nndns", cmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusInternalServerError),
		))
	})

	It("should fail to create NNDN with invalid reason", func() {
		cmd := &request.CreateNNDNRequest{
			Name:   "test1.nndntest",
			Reason: "ab", // too short for ClIDType (min 3 chars)
		}
		resp := api.POST("/nndns", cmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusInternalServerError),
		))
	})

	// ================================================================== //
	//  Create — happy paths                                               //
	// ================================================================== //

	It("should create NNDN 1", func() {
		cmd := &request.CreateNNDNRequest{
			Name:   nndn1,
			Reason: "reserved",
		}
		resp := api.POST("/nndns", cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create NNDN 1 failed: %s", resp.Body.String())

		var nndn entities.NNDN
		Expect(DecodeJSON(resp, &nndn)).To(Succeed())
		Expect(nndn.Name.String()).To(Equal(nndn1))
	})

	It("should create NNDN 2", func() {
		cmd := &request.CreateNNDNRequest{
			Name:   nndn2,
			Reason: "blocked",
		}
		resp := api.POST("/nndns", cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create NNDN 2 failed: %s", resp.Body.String())

		var nndn entities.NNDN
		Expect(DecodeJSON(resp, &nndn)).To(Succeed())
		Expect(nndn.Name.String()).To(Equal(nndn2))
	})

	// ================================================================== //
	//  List — two results                                                 //
	// ================================================================== //

	It("should list 2 NNDNs for the test TLD", func() {
		resp := api.GET(fmt.Sprintf("/nndns?tld_equals=%s", tldName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListNndnsFilter{}
		res.Meta.Filter = &filter
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(Equal(2))
	})

	// ================================================================== //
	//  Count                                                              //
	// ================================================================== //

	It("should count at least 2 NNDNs", func() {
		resp := api.GET("/nndns/count")
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result response.CountResult
		Expect(DecodeJSON(resp, &result)).To(Succeed())
		Expect(result.Count).To(BeNumerically(">=", 2))
	})

	// ================================================================== //
	//  Get — happy and unhappy paths                                      //
	// ================================================================== //

	It("should get NNDN 1 by name", func() {
		resp := api.GET(fmt.Sprintf("/nndns/%s", nndn1))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var nndn entities.NNDN
		Expect(DecodeJSON(resp, &nndn)).To(Succeed())
		Expect(nndn.Name.String()).To(Equal(nndn1))
		Expect(nndn.Reason.String()).To(Equal("reserved"))
	})

	It("should return 404 for non-existent NNDN", func() {
		resp := api.GET(fmt.Sprintf("/nndns/%s", "doesnotexist.nndntest"))
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	// ================================================================== //
	//  Delete — happy and unhappy paths                                   //
	// ================================================================== //

	It("should delete NNDN 1", func() {
		resp := api.DELETE(fmt.Sprintf("/nndns/%s", nndn1))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should handle delete of already-deleted NNDN 1", func() {
		resp := api.DELETE(fmt.Sprintf("/nndns/%s", nndn1))
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusNoContent),
			Equal(http.StatusNotFound),
			Equal(http.StatusInternalServerError),
		))
	})

	It("should return 404 for deleted NNDN 1", func() {
		resp := api.GET(fmt.Sprintf("/nndns/%s", nndn1))
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	// ================================================================== //
	//  List — check residue                                               //
	// ================================================================== //

	It("should list 1 NNDN remaining for the test TLD", func() {
		resp := api.GET(fmt.Sprintf("/nndns?tld_equals=%s", tldName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListNndnsFilter{}
		res.Meta.Filter = &filter
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(Equal(1))
	})

	// ------------------------------------------------------------------ //
	//  AfterAll: best-effort cleanup (no assertions)                      //
	// ------------------------------------------------------------------ //
	AfterAll(func() {
		// NNDNs
		api.DELETE(fmt.Sprintf("/nndns/%s", nndn1))
		api.DELETE(fmt.Sprintf("/nndns/%s", nndn2))

		// TLD
		api.DELETE(fmt.Sprintf("/tlds/%s", tldName))

		// Registry Operator
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID))
	})
})
