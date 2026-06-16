package tests

import (
	"fmt"
	"net/http"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Fee Controller Integration Test
//
// Story: manage phase fees through their full lifecycle
//
//	create → list → delete → verify deletion
var _ = Describe("FeeController", Ordered, func() {
	const (
		ryID      = "feeTestRyOp"
		tldName   = "feetest"
		phaseName = "FeeLnch"
		feeName   = "verification"
		appFee    = "application"
	)

	// ------------------------------------------------------------------ //
	//  Setup: create prerequisite entities                                //
	// ------------------------------------------------------------------ //
	BeforeAll(func() {
		// 1. Registry Operator
		ryCmd := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID,
			Name:  "Fee Test Registry Operator",
			Email: "admin@feetest.com",
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

		// 3. Launch Phase (starts in the past, UTC)
		phaseCmd := &commands.CreatePhaseCommand{
			Name:   phaseName,
			Type:   "Launch",
			Starts: time.Now().UTC().Add(-24 * time.Hour),
		}
		resp = api.POST(fmt.Sprintf("/tlds/%s/phases", tldName), phaseCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	// ================================================================== //
	//  List — zero results                                                //
	// ================================================================== //

	It("should list fees with zero results initially", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases/%s/fees", tldName, phaseName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var fees []entities.Fee
		Expect(DecodeJSON(resp, &fees)).To(Succeed())
		Expect(fees).To(BeEmpty())
	})

	// ================================================================== //
	//  Create — unhappy paths                                             //
	// ================================================================== //

	It("should fail to create fee with no body", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/tlds/%s/phases/%s/fees", tldName, phaseName))
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should fail to create fee with missing currency", func() {
		cmd := map[string]interface{}{
			"name":       feeName,
			"amount":     10000,
			"refundable": false,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/fees", tldName, phaseName), cmd)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should fail to create fee for non-existent TLD", func() {
		cmd := &commands.CreateFeeCommand{
			Name:       feeName,
			Currency:   "USD",
			Amount:     10000,
			Refundable: false,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/fees", "nonexistent", phaseName), cmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusNotFound),
			Equal(http.StatusInternalServerError),
		))
	})

	It("should fail to create fee for non-existent phase", func() {
		cmd := &commands.CreateFeeCommand{
			Name:       feeName,
			Currency:   "USD",
			Amount:     10000,
			Refundable: false,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/fees", tldName, "nonexistent"), cmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusNotFound),
			Equal(http.StatusInternalServerError),
		))
	})

	// ================================================================== //
	//  Create — happy paths                                               //
	// ================================================================== //

	It("should create a verification fee in USD", func() {
		cmd := &commands.CreateFeeCommand{
			Name:       feeName,
			Currency:   "USD",
			Amount:     10000,
			Refundable: false,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/fees", tldName, phaseName), cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create fee USD failed: %s", resp.Body.String())

		var fee entities.Fee
		Expect(DecodeJSON(resp, &fee)).To(Succeed())
		Expect(fee.Currency).To(Equal("USD"))
		Expect(fee.Name.String()).To(Equal(feeName))
		Expect(fee.Amount).To(Equal(uint64(10000)))
	})

	It("should fail to create a duplicate verification fee in USD", func() {
		cmd := &commands.CreateFeeCommand{
			Name:       feeName,
			Currency:   "USD",
			Amount:     10000,
			Refundable: false,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/fees", tldName, phaseName), cmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusConflict),
		))
	})

	It("should create a verification fee in PEN", func() {
		cmd := &commands.CreateFeeCommand{
			Name:       feeName,
			Currency:   "PEN",
			Amount:     50000,
			Refundable: false,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/fees", tldName, phaseName), cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create fee PEN failed: %s", resp.Body.String())

		var fee entities.Fee
		Expect(DecodeJSON(resp, &fee)).To(Succeed())
		Expect(fee.Currency).To(Equal("PEN"))
	})

	// ================================================================== //
	//  List — two fees                                                    //
	// ================================================================== //

	It("should list two fees", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases/%s/fees", tldName, phaseName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var fees []entities.Fee
		Expect(DecodeJSON(resp, &fees)).To(Succeed())
		Expect(len(fees)).To(Equal(2))
	})

	// ================================================================== //
	//  Create — another fee type                                          //
	// ================================================================== //

	It("should create an application fee in USD", func() {
		cmd := &commands.CreateFeeCommand{
			Name:       appFee,
			Currency:   "USD",
			Amount:     20000,
			Refundable: true,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/fees", tldName, phaseName), cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		var fee entities.Fee
		Expect(DecodeJSON(resp, &fee)).To(Succeed())
		Expect(fee.Name.String()).To(Equal(appFee))
	})

	It("should list three fees after adding application fee", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases/%s/fees", tldName, phaseName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var fees []entities.Fee
		Expect(DecodeJSON(resp, &fees)).To(Succeed())
		Expect(len(fees)).To(Equal(3))
	})

	// ================================================================== //
	//  Create — invalid currency                                          //
	// ================================================================== //

	It("should fail to create fee with invalid currency", func() {
		cmd := &commands.CreateFeeCommand{
			Name:       "badcur",
			Currency:   "INVALID",
			Amount:     5000,
			Refundable: false,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/fees", tldName, phaseName), cmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusInternalServerError),
		))
	})

	// ================================================================== //
	//  Delete — happy path                                                //
	// ================================================================== //

	It("should delete the application fee in USD", func() {
		resp := api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s/fees/%s/%s", tldName, phaseName, appFee, "USD"))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should list two fees after deletion", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases/%s/fees", tldName, phaseName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var fees []entities.Fee
		Expect(DecodeJSON(resp, &fees)).To(Succeed())
		Expect(len(fees)).To(Equal(2))
	})

	// ------------------------------------------------------------------ //
	//  AfterAll: best-effort cleanup (no assertions)                      //
	// ------------------------------------------------------------------ //
	AfterAll(func() {
		// Fees (delete before phase)
		api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s/fees/%s/%s", tldName, phaseName, feeName, "USD"))
		api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s/fees/%s/%s", tldName, phaseName, feeName, "PEN"))
		api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s/fees/%s/%s", tldName, phaseName, appFee, "USD"))

		// Phase
		api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s", tldName, phaseName))

		// TLD
		api.DELETE(fmt.Sprintf("/tlds/%s", tldName))

		// Registry Operator
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID))
	})
})
