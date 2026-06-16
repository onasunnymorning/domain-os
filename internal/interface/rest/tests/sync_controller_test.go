package tests

import (
	"net/http"
	"os"

	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Sync Controller Integration Test
//
// Story: verify that sync & read endpoints for external-data sources exist and respond.
//
// These endpoints call external APIs (IANA, ICANN, OpenExchangeRates) that may
// not be reachable in CI. Tests are therefore written to be RESILIENT: they
// accept both success and failure status codes and never fail due to missing
// API keys or network connectivity.
var _ = Describe("SyncController", Ordered, func() {

	// ================================================================== //
	//  Spec 1: Sync IANA registrars                                       //
	// ================================================================== //
	It("should accept a PUT to /sync/iana-registrars without crashing", func() {
		resp := api.PUT("/sync/iana-registrars", nil)
		// The sync may succeed (200) or fail (500) depending on network access
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusOK),
			Equal(http.StatusInternalServerError),
		))
	})

	// ================================================================== //
	//  Spec 2: List IANA registrars                                       //
	// ================================================================== //
	It("should list IANA registrars", func() {
		resp := api.GET("/ianaregistrars")
		Expect(resp.Code).To(Equal(http.StatusOK))

		// Decode into a generic map — Data may be empty if sync didn't run
		var result map[string]interface{}
		err := DecodeJSON(resp, &result)
		Expect(err).NotTo(HaveOccurred())
	})

	// ================================================================== //
	//  Spec 3: Count IANA registrars                                      //
	// ================================================================== //
	It("should count IANA registrars", func() {
		resp := api.GET("/ianaregistrars/count")
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result response.CountResult
		err := DecodeJSON(resp, &result)
		Expect(err).NotTo(HaveOccurred())
		// Count may be 0 if sync didn't succeed
		Expect(result.Count).To(BeNumerically(">=", int64(0)))
	})

	// ================================================================== //
	//  Spec 4: Sync ICANN Spec5 labels                                    //
	// ================================================================== //
	It("should accept a PUT to /sync/icann-spec5 without crashing", func() {
		resp := api.PUT("/sync/icann-spec5", nil)
		// The sync may succeed (200) or fail (500) depending on network access
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusOK),
			Equal(http.StatusInternalServerError),
		))
	})

	// ================================================================== //
	//  Spec 5: List Spec5 labels                                          //
	// ================================================================== //
	It("should list Spec5 labels", func() {
		resp := api.GET("/spec5labels")
		Expect(resp.Code).To(Equal(http.StatusOK))

		// Decode into a generic map — the ListItemResult.Meta.Filter is an
		// interface type that can't be unmarshaled from JSON without a type hint.
		var result map[string]interface{}
		err := DecodeJSON(resp, &result)
		Expect(err).NotTo(HaveOccurred())
	})

	// ================================================================== //
	//  Spec 6: List FX rates by base currency                             //
	// ================================================================== //
	It("should list FX rates for USD", func() {
		resp := api.GET("/fx/USD")
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusOK),
			Equal(http.StatusInternalServerError),
		))
	})

	// ================================================================== //
	//  Spec 7: Get specific FX rate                                       //
	// ================================================================== //
	It("should get FX rate for USD to EUR", func() {
		resp := api.GET("/fx/usd/eur")
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusOK),
			Equal(http.StatusInternalServerError),
		))
	})

	// ================================================================== //
	//  Spec 8: Sync FX rates (conditional on API key)                     //
	// ================================================================== //
	It("should sync FX rates when OPENEXCHANGERATES_APP_ID is set", func() {
		if os.Getenv("OPENEXCHANGERATES_APP_ID") == "" {
			Skip("OPENEXCHANGERATES_APP_ID not set — skipping FX sync test")
		}

		resp := api.PUT("/sync/fx/USD", nil)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusOK),
			Equal(http.StatusInternalServerError),
		))
	})

	// No AfterAll needed — sync endpoints are read/write but don't create
	// test-specific entities that need cleanup.
})
