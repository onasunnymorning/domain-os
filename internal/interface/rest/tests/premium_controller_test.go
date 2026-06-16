package tests

import (
	"fmt"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Premium Controller Integration Test
//
// Story: manage Premium Lists and Labels through their full lifecycle
//
//	create list → list → get → delete → verify deletion
//	create labels → list → duplicate check → cleanup
var _ = Describe("PremiumController", Ordered, func() {
	const (
		ryID      = "prmTestRyOp"
		list1Name = "prmlist1"
		list2Name = "prmlist2"
		labelName = "premium"
	)

	// ------------------------------------------------------------------ //
	//  Setup: create prerequisite entities                                //
	// ------------------------------------------------------------------ //
	BeforeAll(func() {
		// Registry Operator
		ryCmd := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID,
			Name:  "Premium Test RyOp",
			Email: "admin@prmtest.com",
		}
		resp := api.POST("/registry-operators", ryCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	// ================================================================== //
	//  Premium Lists Story                                                //
	// ================================================================== //

	It("should create premium list 1", func() {
		cmd := &commands.CreatePremiumListCommand{
			Name: list1Name,
			RyID: ryID,
		}
		resp := api.POST("/premium/lists", cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create list 1 failed: %s", resp.Body.String())

		var list entities.PremiumList
		Expect(DecodeJSON(resp, &list)).To(Succeed())
		Expect(list.Name).To(Equal(list1Name))
	})

	It("should create premium list 2", func() {
		cmd := &commands.CreatePremiumListCommand{
			Name: list2Name,
			RyID: ryID,
		}
		resp := api.POST("/premium/lists", cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create list 2 failed: %s", resp.Body.String())

		var list entities.PremiumList
		Expect(DecodeJSON(resp, &list)).To(Succeed())
		Expect(list.Name).To(Equal(list2Name))
	})

	It("should list at least 2 premium lists", func() {
		resp := api.GET("/premium/lists")
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListPremiumListsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(BeNumerically(">=", 2))
	})

	It("should get premium list 1 by name", func() {
		resp := api.GET(fmt.Sprintf("/premium/lists/%s", list1Name))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var list entities.PremiumList
		Expect(DecodeJSON(resp, &list)).To(Succeed())
		Expect(list.Name).To(Equal(list1Name))
		Expect(list.RyID.String()).To(Equal(ryID))
	})

	It("should delete premium list 1", func() {
		resp := api.DELETE(fmt.Sprintf("/premium/lists/%s", list1Name))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should handle delete of already-deleted premium list 1", func() {
		resp := api.DELETE(fmt.Sprintf("/premium/lists/%s", list1Name))
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusNoContent),
			Equal(http.StatusNotFound),
			Equal(http.StatusInternalServerError),
		))
	})

	It("should return 404 for deleted premium list 1", func() {
		resp := api.GET(fmt.Sprintf("/premium/lists/%s", list1Name))
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	// ================================================================== //
	//  Premium Labels Story (on list 2)                                   //
	// ================================================================== //

	It("should list labels with zero results for list 2", func() {
		resp := api.GET(fmt.Sprintf("/premium/labels?premium_list_name_equals=%s", list2Name))
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListPremiumLabelsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		if res.Data != nil {
			itemSlice, ok := res.Data.([]interface{})
			if ok {
				Expect(itemSlice).To(BeEmpty())
			}
		}
	})

	It("should create a premium label in USD", func() {
		cmd := &commands.CreatePremiumLabelCommand{
			Label:              labelName,
			PremiumListName:    list2Name,
			RegistrationAmount: 10000,
			RenewalAmount:      10000,
			TransferAmount:     5000,
			RestoreAmount:      20000,
			Currency:           "USD",
			Class:              "gold",
		}
		resp := api.POST(fmt.Sprintf("/premium/lists/%s/labels", list2Name), cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create label USD failed: %s", resp.Body.String())

		var label entities.PremiumLabel
		Expect(DecodeJSON(resp, &label)).To(Succeed())
		Expect(label.Currency).To(Equal("USD"))
	})

	It("should create a premium label in PEN", func() {
		cmd := &commands.CreatePremiumLabelCommand{
			Label:              labelName,
			PremiumListName:    list2Name,
			RegistrationAmount: 50000,
			RenewalAmount:      50000,
			TransferAmount:     25000,
			RestoreAmount:      100000,
			Currency:           "PEN",
			Class:              "gold",
		}
		resp := api.POST(fmt.Sprintf("/premium/lists/%s/labels", list2Name), cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create label PEN failed: %s", resp.Body.String())

		var label entities.PremiumLabel
		Expect(DecodeJSON(resp, &label)).To(Succeed())
		Expect(label.Currency).To(Equal("PEN"))
	})

	It("should fail to create duplicate premium label", func() {
		cmd := &commands.CreatePremiumLabelCommand{
			Label:              labelName,
			PremiumListName:    list2Name,
			RegistrationAmount: 10000,
			RenewalAmount:      10000,
			TransferAmount:     5000,
			RestoreAmount:      20000,
			Currency:           "USD",
			Class:              "gold",
		}
		resp := api.POST(fmt.Sprintf("/premium/lists/%s/labels", list2Name), cmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusConflict),
			Equal(http.StatusInternalServerError),
		))
	})

	It("should list 2 labels for list 2", func() {
		resp := api.GET(fmt.Sprintf("/premium/labels?premium_list_name_equals=%s", list2Name))
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListPremiumLabelsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(Equal(2))
	})

	It("should get premium label by composite key", func() {
		resp := api.GET(fmt.Sprintf("/premium/lists/%s/labels/%s/%s", list2Name, labelName, "USD"))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var label entities.PremiumLabel
		Expect(DecodeJSON(resp, &label)).To(Succeed())
		Expect(label.Currency).To(Equal("USD"))
		Expect(label.PremiumListName).To(Equal(list2Name))
	})

	// ------------------------------------------------------------------ //
	//  AfterAll: best-effort cleanup (no assertions)                      //
	// ------------------------------------------------------------------ //
	AfterAll(func() {
		// Labels
		api.DELETE(fmt.Sprintf("/premium/lists/%s/labels/%s/%s", list2Name, labelName, "USD"))
		api.DELETE(fmt.Sprintf("/premium/lists/%s/labels/%s/%s", list2Name, labelName, "PEN"))

		// Lists
		api.DELETE(fmt.Sprintf("/premium/lists/%s", list2Name))
		api.DELETE(fmt.Sprintf("/premium/lists/%s", list1Name))

		// Registry Operator
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID))
	})
})
