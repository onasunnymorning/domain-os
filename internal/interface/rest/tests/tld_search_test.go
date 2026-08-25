package tests

import (
	"fmt"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TLD Search Integration Test
//
// Story: exercise TLD listing with filters and pagination
//
//	create 10 TLDs across types and registry operators → filter → paginate → count → cleanup
var _ = Describe("TLDSearch", Ordered, func() {
	const (
		ryID1 = "srchRyOp1"
		ryID2 = "srchRyOp2"
	)

	// TLD names grouped by type
	// ccTLDs (exactly 2 chars): aa, bb
	// SLDs (contain a dot):     co.aa, co.bb, co.cc
	// gTLDs (>2 chars, no dot): srchone, srchtwo, srchthree, srchfour, srchfive
	type tldDef struct {
		name string
		ryID string
	}

	tlds := []tldDef{
		// ccTLDs — RyOp1
		{"aa", ryID1},
		{"bb", ryID1},
		// SLDs — mixed
		{"co.aa", ryID1},
		{"co.bb", ryID1},
		{"co.cc", ryID2},
		// gTLDs — mixed
		{"srchone", ryID1},
		{"srchtwo", ryID2},
		{"srchthree", ryID2},
		{"srchfour", ryID2},
		{"srchfive", ryID2},
	}

	// ------------------------------------------------------------------ //
	//  Setup: create prerequisite entities                                //
	// ------------------------------------------------------------------ //
	BeforeAll(func() {
		// 1. Registry Operator 1
		ryCmd1 := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID1,
			Name:  "Search Test Registry Operator 1",
			Email: "admin@srchtest1.com",
		}
		resp := api.POST("/registry-operators", ryCmd1)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 2. Registry Operator 2
		ryCmd2 := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID2,
			Name:  "Search Test Registry Operator 2",
			Email: "admin@srchtest2.com",
		}
		resp = api.POST("/registry-operators", ryCmd2)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 3. Create all 10 TLDs
		for _, t := range tlds {
			tldReq := &request.CreateTLDRequest{
				Name: t.name,
				RyID: t.ryID,
			}
			resp = api.POST("/tlds", tldReq)
			Expect(resp.Code).To(Equal(http.StatusCreated), "Failed to create TLD %s: %s", t.name, resp.Body.String())
		}
	})

	// ================================================================== //
	//  Spec 1: List all TLDs — should find all created ones              //
	// ================================================================== //
	It("should list TLDs and find all created ones", func() {
		resp := api.GET("/tlds")
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListTldsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(BeNumerically(">=", 10))
	})

	// ================================================================== //
	//  Spec 2: Count TLDs                                                //
	// ================================================================== //
	It("should count TLDs", func() {
		resp := api.GET("/tlds/count")
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result response.CountResult
		err := DecodeJSON(resp, &result)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Count).To(BeNumerically(">=", int64(10)))
	})

	// ================================================================== //
	//  Spec 3: List with type filter (ccTLD)                             //
	// ================================================================== //
	It("should list only country-code TLDs when type_equals=country-code", func() {
		resp := api.GET("/tlds?type_equals=country-code")
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListTldsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		// We created 2 ccTLDs: aa, bb
		Expect(len(itemSlice)).To(BeNumerically(">=", 2))
	})

	// ================================================================== //
	//  Spec 4: Count with type filter                                    //
	// ================================================================== //
	It("should count only country-code TLDs", func() {
		resp := api.GET("/tlds/count?type_equals=country-code")
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result response.CountResult
		err := DecodeJSON(resp, &result)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Count).To(BeNumerically(">=", int64(2)))
	})

	// ================================================================== //
	//  Spec 5: List with ryid filter                                     //
	// ================================================================== //
	It("should list only TLDs for srchRyOp1 when ryid_equals is set", func() {
		resp := api.GET(fmt.Sprintf("/tlds?ryid_equals=%s", ryID1))
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListTldsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		// RyOp1 has: aa, bb, co.aa, co.bb, srchone = 5 TLDs
		Expect(len(itemSlice)).To(BeNumerically(">=", 5))
	})

	// ================================================================== //
	//  Spec 6: Count with ryid filter                                    //
	// ================================================================== //
	It("should count only TLDs for srchRyOp1", func() {
		resp := api.GET(fmt.Sprintf("/tlds/count?ryid_equals=%s", ryID1))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result response.CountResult
		err := DecodeJSON(resp, &result)
		Expect(err).NotTo(HaveOccurred())
		// RyOp1 has 5 TLDs
		Expect(result.Count).To(BeNumerically(">=", int64(5)))
	})

	// ================================================================== //
	//  Spec 7: List with pagination                                      //
	// ================================================================== //
	It("should respect pagesize=3 on listing", func() {
		resp := api.GET("/tlds?pagesize=3")
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListTldsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(Equal(3))
		Expect(res.Meta.PageSize).To(Equal(3))
	})

	// ================================================================== //
	//  Spec 8: List with name_like filter                                //
	// ================================================================== //
	It("should list TLDs matching name_like=srch", func() {
		resp := api.GET("/tlds?name_like=srch")
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListTldsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		// srchone, srchtwo, srchthree, srchfour, srchfive = 5 matches
		Expect(len(itemSlice)).To(BeNumerically(">=", 5))
	})

	// ================================================================== //
	//  Spec 9: Count with name_like filter                               //
	// ================================================================== //
	It("should count TLDs matching name_like=srch", func() {
		resp := api.GET("/tlds/count?name_like=srch")
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result response.CountResult
		err := DecodeJSON(resp, &result)
		Expect(err).NotTo(HaveOccurred())
		// 5 gTLDs with "srch" prefix
		Expect(result.Count).To(BeNumerically(">=", int64(5)))
	})

	// ================================================================== //
	//  Spec 10: List with combined filters (name_like + ryid)            //
	// ================================================================== //
	It("should list TLDs matching name_like=srch AND ryid_equals=srchRyOp2", func() {
		resp := api.GET(fmt.Sprintf("/tlds?name_like=srch&ryid_equals=%s", ryID2))
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListTldsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		// RyOp2 gTLDs with "srch": srchtwo, srchthree, srchfour, srchfive = 4
		Expect(len(itemSlice)).To(BeNumerically(">=", 4))
	})

	// ------------------------------------------------------------------ //
	//  AfterAll: best-effort cleanup (no assertions)                      //
	// ------------------------------------------------------------------ //
	AfterAll(func() {
		// Delete all TLDs (reverse order doesn't matter, but SLDs reference ccTLDs
		// so delete SLDs first)
		for i := len(tlds) - 1; i >= 0; i-- {
			api.DELETE(fmt.Sprintf("/tlds/%s", tlds[i].name))
		}

		// Delete Registry Operators
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID1))
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID2))
	})
})
