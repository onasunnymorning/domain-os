package tests

import (
	"fmt"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Accreditation Controller Integration Test
//
// Story: manage TLD ↔ Registrar accreditations through their full lifecycle
//
//	accredit → check → list → de-accredit → verify removal
var _ = Describe("AccreditationController", Ordered, func() {
	const (
		ryID     = "accTestRyOp"
		gTLDName = "accgtld"  // 7 chars → generic TLD
		ccTLDName = "ac"      // 2 chars → country-code TLD
		rar1ClID = "accTestRar1"
		rar2ClID = "accTestRar2"
		rar1GurID = 10009
		rar2GurID = 10010
	)

	// ------------------------------------------------------------------ //
	//  Setup: create prerequisite entities                                //
	// ------------------------------------------------------------------ //
	BeforeAll(func() {
		// 1. Registry Operator
		ryCmd := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID,
			Name:  "Accreditation Test RyOp",
			Email: "admin@acctest.com",
		}
		resp := api.POST("/registry-operators", ryCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 2. gTLD
		gTLDReq := &request.CreateTLDRequest{
			Name: gTLDName,
			RyID: ryID,
		}
		resp = api.POST("/tlds", gTLDReq)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 3. ccTLD
		ccTLDReq := &request.CreateTLDRequest{
			Name: ccTLDName,
			RyID: ryID,
		}
		resp = api.POST("/tlds", ccTLDReq)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 4. Registrar 1
		rar1Payload := testRegistrar(rar1ClID, "Acc Test Registrar 1", rar1GurID)
		resp = api.POST("/registrars", rar1Payload)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 5. Registrar 2
		rar2Payload := testRegistrar(rar2ClID, "Acc Test Registrar 2", rar2GurID)
		resp = api.POST("/registrars", rar2Payload)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		// 6. Set Registrar 1 status to ok + IANA Accredited
		resp = api.PUT(fmt.Sprintf("/registrars/%s/status/ok", rar1ClID), nil)
		Expect(resp.Code).To(SatisfyAny(Equal(http.StatusOK), Equal(http.StatusNoContent)))
		resp = api.PUT(fmt.Sprintf("/registrars/%s/iana_status/Accredited", rar1ClID), nil)
		Expect(resp.Code).To(SatisfyAny(Equal(http.StatusOK), Equal(http.StatusNoContent)))

		// 7. Set Registrar 2 status to ok + IANA Accredited
		resp = api.PUT(fmt.Sprintf("/registrars/%s/status/ok", rar2ClID), nil)
		Expect(resp.Code).To(SatisfyAny(Equal(http.StatusOK), Equal(http.StatusNoContent)))
		resp = api.PUT(fmt.Sprintf("/registrars/%s/iana_status/Accredited", rar2ClID), nil)
		Expect(resp.Code).To(SatisfyAny(Equal(http.StatusOK), Equal(http.StatusNoContent)))
	})

	// ================================================================== //
	//  List — zero results initially                                      //
	// ================================================================== //

	It("should list gTLD accreditations with zero results", func() {
		resp := api.GET(fmt.Sprintf("/accreditations/tld/%s", gTLDName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var res response.ListItemResult
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		// Data may be nil or empty slice
		if res.Data != nil {
			itemSlice, ok := res.Data.([]interface{})
			if ok {
				Expect(itemSlice).To(BeEmpty())
			}
		}
	})

	It("should list registrar 1 accreditations with zero results", func() {
		resp := api.GET(fmt.Sprintf("/accreditations/registrar/%s", rar1ClID))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var res response.ListItemResult
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		if res.Data != nil {
			itemSlice, ok := res.Data.([]interface{})
			if ok {
				Expect(itemSlice).To(BeEmpty())
			}
		}
	})

	// ================================================================== //
	//  IsAccredited — initially NO                                        //
	// ================================================================== //

	It("should report registrar 1 as NOT accredited for gTLD", func() {
		resp := api.GET(fmt.Sprintf("/accreditations/%s/%s", gTLDName, rar1ClID))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result response.IsAccreditedResponse
		Expect(DecodeJSON(resp, &result)).To(Succeed())
		Expect(result.IsAccredited).To(BeFalse())
	})

	// ================================================================== //
	//  IsAccredited — unhappy paths                                       //
	// ================================================================== //

	It("should handle lookup for non-existing registrar", func() {
		resp := api.GET(fmt.Sprintf("/accreditations/%s/%s", gTLDName, "nonexistent"))
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusOK),
			Equal(http.StatusBadRequest),
			Equal(http.StatusNotFound),
		))
		// If 200, verify IsAccredited is false
		if resp.Code == http.StatusOK {
			var result response.IsAccreditedResponse
			Expect(DecodeJSON(resp, &result)).To(Succeed())
			Expect(result.IsAccredited).To(BeFalse())
		}
	})

	It("should handle lookup for non-existing TLD", func() {
		resp := api.GET(fmt.Sprintf("/accreditations/%s/%s", "nonexistent", rar1ClID))
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusOK),
			Equal(http.StatusBadRequest),
			Equal(http.StatusNotFound),
		))
		// If 200, verify IsAccredited is false
		if resp.Code == http.StatusOK {
			var result response.IsAccreditedResponse
			Expect(DecodeJSON(resp, &result)).To(Succeed())
			Expect(result.IsAccredited).To(BeFalse())
		}
	})

	// ================================================================== //
	//  Accredit — happy paths                                             //
	// ================================================================== //

	It("should accredit registrar 1 for gTLD", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/accreditations/%s/%s", gTLDName, rar1ClID))
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	It("should accredit registrar 1 for ccTLD", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/accreditations/%s/%s", ccTLDName, rar1ClID))
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	// ================================================================== //
	//  IsAccredited — now YES                                             //
	// ================================================================== //

	It("should report registrar 1 as accredited for gTLD", func() {
		resp := api.GET(fmt.Sprintf("/accreditations/%s/%s", gTLDName, rar1ClID))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result response.IsAccreditedResponse
		Expect(DecodeJSON(resp, &result)).To(Succeed())
		Expect(result.IsAccredited).To(BeTrue())
	})

	// ================================================================== //
	//  List — after accreditations                                        //
	// ================================================================== //

	It("should list 1 registrar accredited for gTLD", func() {
		resp := api.GET(fmt.Sprintf("/accreditations/tld/%s", gTLDName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var res response.ListItemResult
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(Equal(1))
	})

	It("should list 2 TLDs for registrar 1", func() {
		resp := api.GET(fmt.Sprintf("/accreditations/registrar/%s", rar1ClID))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var res response.ListItemResult
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(Equal(2))
	})

	It("should accredit registrar 2 for ccTLD", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/accreditations/%s/%s", ccTLDName, rar2ClID))
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	It("should list 2 registrars accredited for ccTLD", func() {
		resp := api.GET(fmt.Sprintf("/accreditations/tld/%s", ccTLDName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var res response.ListItemResult
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(Equal(2))
	})

	// ================================================================== //
	//  De-accredit — happy and unhappy paths                              //
	// ================================================================== //

	It("should de-accredit registrar 1 from gTLD", func() {
		resp := api.DELETE(fmt.Sprintf("/accreditations/%s/%s", gTLDName, rar1ClID))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should handle de-accredit of already removed accreditation", func() {
		resp := api.DELETE(fmt.Sprintf("/accreditations/%s/%s", gTLDName, rar1ClID))
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusNoContent),
			Equal(http.StatusBadRequest),
			Equal(http.StatusNotFound),
			Equal(http.StatusInternalServerError),
		))
	})

	It("should report registrar 1 as NOT accredited for gTLD after de-accreditation", func() {
		resp := api.GET(fmt.Sprintf("/accreditations/%s/%s", gTLDName, rar1ClID))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result response.IsAccreditedResponse
		Expect(DecodeJSON(resp, &result)).To(Succeed())
		Expect(result.IsAccredited).To(BeFalse())
	})

	// ------------------------------------------------------------------ //
	//  AfterAll: best-effort cleanup (no assertions)                      //
	// ------------------------------------------------------------------ //
	AfterAll(func() {
		// Accreditations
		api.DELETE(fmt.Sprintf("/accreditations/%s/%s", gTLDName, rar1ClID))
		api.DELETE(fmt.Sprintf("/accreditations/%s/%s", ccTLDName, rar1ClID))
		api.DELETE(fmt.Sprintf("/accreditations/%s/%s", ccTLDName, rar2ClID))

		// Registrars
		api.DELETE(fmt.Sprintf("/registrars/%s", rar1ClID))
		api.DELETE(fmt.Sprintf("/registrars/%s", rar2ClID))

		// TLDs
		api.DELETE(fmt.Sprintf("/tlds/%s", gTLDName))
		api.DELETE(fmt.Sprintf("/tlds/%s", ccTLDName))

		// Registry Operator
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID))
	})
})
