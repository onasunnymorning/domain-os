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

// Price Controller Integration Test
//
// Story: manage phase prices through their full lifecycle
//
//	create → list → delete → verify deletion
var _ = Describe("PriceController", Ordered, func() {
	const (
		ryID      = "prcTestRyOp"
		tldName   = "prctest"
		phaseName = "PrcLnch"
	)

	// ------------------------------------------------------------------ //
	//  Setup: create prerequisite entities                                //
	// ------------------------------------------------------------------ //
	BeforeAll(func() {
		// 1. Registry Operator
		ryCmd := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID,
			Name:  "Price Test Registry Operator",
			Email: "admin@prctest.com",
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

	It("should list prices with zero results initially", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases/%s/prices", tldName, phaseName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var prices []entities.Price
		Expect(DecodeJSON(resp, &prices)).To(Succeed())
		Expect(prices).To(BeEmpty())
	})

	// ================================================================== //
	//  Create — unhappy paths                                             //
	// ================================================================== //

	It("should fail to create price with no body", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/tlds/%s/phases/%s/prices", tldName, phaseName))
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should fail to create price with missing currency", func() {
		cmd := map[string]interface{}{
			"registrationAmount": 1000,
			"renewalAmount":      1000,
			"transferAmount":     1000,
			"restoreAmount":      5000,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/prices", tldName, phaseName), cmd)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should fail to create price for non-existent TLD", func() {
		cmd := &commands.CreatePriceCommand{
			Currency:           "USD",
			RegistrationAmount: 1000,
			RenewalAmount:      1000,
			TransferAmount:     1000,
			RestoreAmount:      5000,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/prices", "nonexistent", phaseName), cmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusNotFound),
			Equal(http.StatusInternalServerError),
		))
	})

	It("should fail to create price for non-existent phase", func() {
		cmd := &commands.CreatePriceCommand{
			Currency:           "USD",
			RegistrationAmount: 1000,
			RenewalAmount:      1000,
			TransferAmount:     1000,
			RestoreAmount:      5000,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/prices", tldName, "nonexistent"), cmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusNotFound),
			Equal(http.StatusInternalServerError),
		))
	})

	// ================================================================== //
	//  Create — happy paths                                               //
	// ================================================================== //

	It("should create a price in USD", func() {
		cmd := &commands.CreatePriceCommand{
			Currency:           "USD",
			RegistrationAmount: 1000,
			RenewalAmount:      1000,
			TransferAmount:     1000,
			RestoreAmount:      5000,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/prices", tldName, phaseName), cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create price USD failed: %s", resp.Body.String())

		var price entities.Price
		Expect(DecodeJSON(resp, &price)).To(Succeed())
		Expect(price.Currency).To(Equal("USD"))
		Expect(price.RegistrationAmount).To(Equal(uint64(1000)))
		Expect(price.RenewalAmount).To(Equal(uint64(1000)))
		Expect(price.TransferAmount).To(Equal(uint64(1000)))
		Expect(price.RestoreAmount).To(Equal(uint64(5000)))
	})

	It("should fail to create a duplicate price in USD", func() {
		cmd := &commands.CreatePriceCommand{
			Currency:           "USD",
			RegistrationAmount: 1000,
			RenewalAmount:      1000,
			TransferAmount:     1000,
			RestoreAmount:      5000,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/prices", tldName, phaseName), cmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusConflict),
		))
	})

	It("should create a price in PEN", func() {
		cmd := &commands.CreatePriceCommand{
			Currency:           "PEN",
			RegistrationAmount: 5000,
			RenewalAmount:      5000,
			TransferAmount:     5000,
			RestoreAmount:      15000,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/prices", tldName, phaseName), cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create price PEN failed: %s", resp.Body.String())

		var price entities.Price
		Expect(DecodeJSON(resp, &price)).To(Succeed())
		Expect(price.Currency).To(Equal("PEN"))
	})

	// ================================================================== //
	//  List — two prices                                                  //
	// ================================================================== //

	It("should list two prices", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases/%s/prices", tldName, phaseName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var prices []entities.Price
		Expect(DecodeJSON(resp, &prices)).To(Succeed())
		Expect(len(prices)).To(Equal(2))
	})

	// ================================================================== //
	//  Create — another currency                                          //
	// ================================================================== //

	It("should create a price in EUR", func() {
		cmd := &commands.CreatePriceCommand{
			Currency:           "EUR",
			RegistrationAmount: 900,
			RenewalAmount:      900,
			TransferAmount:     900,
			RestoreAmount:      4500,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/prices", tldName, phaseName), cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		var price entities.Price
		Expect(DecodeJSON(resp, &price)).To(Succeed())
		Expect(price.Currency).To(Equal("EUR"))
	})

	It("should list three prices after adding EUR", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases/%s/prices", tldName, phaseName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var prices []entities.Price
		Expect(DecodeJSON(resp, &prices)).To(Succeed())
		Expect(len(prices)).To(Equal(3))
	})

	// ================================================================== //
	//  Create — invalid currency                                          //
	// ================================================================== //

	It("should fail to create price with invalid currency", func() {
		cmd := &commands.CreatePriceCommand{
			Currency:           "INVALID",
			RegistrationAmount: 1000,
			RenewalAmount:      1000,
			TransferAmount:     1000,
			RestoreAmount:      5000,
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases/%s/prices", tldName, phaseName), cmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusInternalServerError),
		))
	})

	// ================================================================== //
	//  Delete — happy path                                                //
	// ================================================================== //

	It("should delete the EUR price", func() {
		resp := api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s/prices/%s", tldName, phaseName, "EUR"))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should list two prices after deletion", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases/%s/prices", tldName, phaseName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var prices []entities.Price
		Expect(DecodeJSON(resp, &prices)).To(Succeed())
		Expect(len(prices)).To(Equal(2))
	})

	// ------------------------------------------------------------------ //
	//  AfterAll: best-effort cleanup (no assertions)                      //
	// ------------------------------------------------------------------ //
	AfterAll(func() {
		// Prices (delete before phase)
		api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s/prices/%s", tldName, phaseName, "USD"))
		api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s/prices/%s", tldName, phaseName, "PEN"))
		api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s/prices/%s", tldName, phaseName, "EUR"))

		// Phase
		api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s", tldName, phaseName))

		// TLD
		api.DELETE(fmt.Sprintf("/tlds/%s", tldName))

		// Registry Operator
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID))
	})
})
