package tests

import (
	"fmt"
	"net/http"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Phase Controller Integration Test
//
// Story: manage TLD phases through their full lifecycle
//
//	create → list → get → set premium list → update policy → end → delete
var _ = Describe("PhaseController", Ordered, func() {
	const (
		ryID       = "phsTestRyOp"
		tldName    = "phstest"
		plName     = "phsTestPL"
		launchName = "PhsLnch"
		gaName     = "PhsGA1"
	)

	// State captured across specs
	var (
		createdLaunchPhase entities.Phase
		createdGAPhase     entities.Phase
	)

	// ------------------------------------------------------------------ //
	//  Setup: create prerequisite entities                                //
	// ------------------------------------------------------------------ //
	BeforeAll(func() {
		// 1. Registry Operator
		ryCmd := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID,
			Name:  "Phase Test Registry Operator",
			Email: "admin@phstest.com",
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

		// 3. Premium List
		plCmd := &commands.CreatePremiumListCommand{
			Name: plName,
			RyID: ryID,
		}
		resp = api.POST("/premium/lists", plCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	// ================================================================== //
	//  List — zero results                                                //
	// ================================================================== //

	It("should list phases with zero results initially", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases", tldName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var res response.ListItemResult
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		// Data should be nil or empty when no phases exist
		if res.Data != nil {
			items, ok := res.Data.([]interface{})
			Expect(ok).To(BeTrue())
			Expect(items).To(BeEmpty())
		}
	})

	// ================================================================== //
	//  Create — unhappy paths                                             //
	// ================================================================== //

	It("should fail to create phase with no body", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/tlds/%s/phases", tldName))
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should fail to create phase with missing type", func() {
		cmd := map[string]interface{}{
			"name":   "NoType",
			"starts": time.Now().UTC().Add(-24 * time.Hour),
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases", tldName), cmd)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should fail to create phase for non-existent TLD", func() {
		cmd := &commands.CreatePhaseCommand{
			Name:   "NoTLD",
			Type:   "GA",
			Starts: time.Now().UTC().Add(-24 * time.Hour),
		}
		resp := api.POST("/tlds/nonexistent/phases", cmd)
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	It("should fail to create phase with invalid type", func() {
		cmd := &commands.CreatePhaseCommand{
			Name:   "BadType",
			Type:   "InvalidType",
			Starts: time.Now().UTC().Add(-24 * time.Hour),
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases", tldName), cmd)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	// ================================================================== //
	//  Create — happy paths                                               //
	// ================================================================== //

	It("should create a Launch phase", func() {
		cmd := &commands.CreatePhaseCommand{
			Name:   launchName,
			Type:   "Launch",
			Starts: time.Now().UTC().Add(-48 * time.Hour),
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases", tldName), cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create Launch phase failed: %s", resp.Body.String())

		Expect(DecodeJSON(resp, &createdLaunchPhase)).To(Succeed())
		Expect(createdLaunchPhase.Name.String()).To(Equal(launchName))
		Expect(string(createdLaunchPhase.Type)).To(Equal("Launch"))
	})

	It("should create a GA phase", func() {
		cmd := &commands.CreatePhaseCommand{
			Name:   gaName,
			Type:   "GA",
			Starts: time.Now().UTC().Add(-24 * time.Hour),
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases", tldName), cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create GA phase failed: %s", resp.Body.String())

		Expect(DecodeJSON(resp, &createdGAPhase)).To(Succeed())
		Expect(createdGAPhase.Name.String()).To(Equal(gaName))
		Expect(string(createdGAPhase.Type)).To(Equal("GA"))
	})

	It("should fail to create an overlapping GA phase", func() {
		cmd := &commands.CreatePhaseCommand{
			Name:   "PhsGA2",
			Type:   "GA",
			Starts: time.Now().UTC().Add(-12 * time.Hour),
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases", tldName), cmd)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusInternalServerError),
		))
	})

	// ================================================================== //
	//  List — active phases                                               //
	// ================================================================== //

	It("should list active phases", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases/active", tldName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var res response.ListItemResult
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		items, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(items)).To(BeNumerically(">=", 2))
	})

	It("should list active GA phases", func() {
		resp := api.GET("/phases/active/ga")
		Expect(resp.Code).To(Equal(http.StatusOK))

		var res response.ListItemResult
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		items, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(items)).To(BeNumerically(">=", 1))
	})

	// ================================================================== //
	//  Get                                                                //
	// ================================================================== //

	It("should get a phase by TLD and name", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases/%s", tldName, gaName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var phase entities.Phase
		Expect(DecodeJSON(resp, &phase)).To(Succeed())
		Expect(phase.Name.String()).To(Equal(gaName))
		Expect(string(phase.Type)).To(Equal("GA"))
	})

	// ================================================================== //
	//  Premium List management                                            //
	// ================================================================== //

	It("should set a premium list on the GA phase", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/tlds/%s/phases/%s/premium-list/%s", tldName, gaName, plName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var phase entities.Phase
		Expect(DecodeJSON(resp, &phase)).To(Succeed())
		Expect(phase.PremiumListName).NotTo(BeNil())
		Expect(*phase.PremiumListName).To(Equal(plName))
	})

	It("should verify premium list is set on GET", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases/%s", tldName, gaName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var phase entities.Phase
		Expect(DecodeJSON(resp, &phase)).To(Succeed())
		Expect(phase.PremiumListName).NotTo(BeNil())
		Expect(*phase.PremiumListName).To(Equal(plName))
	})

	It("should unset the premium list from the GA phase", func() {
		resp := api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s/premium-list/%s", tldName, gaName, plName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var phase entities.Phase
		Expect(DecodeJSON(resp, &phase)).To(Succeed())
		Expect(phase.PremiumListName).To(BeNil())
	})

	// ================================================================== //
	//  Update phase policy                                                //
	// ================================================================== //

	It("should update phase policy", func() {
		newMinLabel := 3
		policyCmd := &commands.UpdatePhasePolicyCommand{
			Policy: &entities.PhasePolicy{
				MinLabelLength: newMinLabel,
			},
		}
		resp := api.PUT(fmt.Sprintf("/tlds/%s/phases/%s/policy", tldName, gaName), policyCmd)
		Expect(resp.Code).To(Equal(http.StatusOK))

		var phase entities.Phase
		Expect(DecodeJSON(resp, &phase)).To(Succeed())
		Expect(phase.Policy.MinLabelLength).To(Equal(newMinLabel))
	})

	// ================================================================== //
	//  End phase                                                          //
	// ================================================================== //

	It("should end the launch phase", func() {
		endCmd := &commands.EndPhaseCommand{
			Ends: time.Now().UTC().Add(1 * time.Hour),
		}
		resp := api.PUT(fmt.Sprintf("/tlds/%s/phases/%s/end", tldName, launchName), endCmd)
		Expect(resp.Code).To(Equal(http.StatusOK))

		var phase entities.Phase
		Expect(DecodeJSON(resp, &phase)).To(Succeed())
		Expect(phase.Ends).NotTo(BeNil())
	})

	// ================================================================== //
	//  Delete — unhappy + happy + verify                                  //
	// ================================================================== //

	It("should fail to delete a currently active phase", func() {
		resp := api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s", tldName, gaName))
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should return 404 when getting a non-existent phase", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases/%s", tldName, "doesNotExist"))
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	It("should list phases and find at least 2", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s/phases", tldName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var res response.ListItemResult
		Expect(DecodeJSON(resp, &res)).To(Succeed())
		items, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(items)).To(BeNumerically(">=", 2))
	})

	// ------------------------------------------------------------------ //
	//  AfterAll: best-effort cleanup (no assertions)                      //
	// ------------------------------------------------------------------ //
	AfterAll(func() {
		// Phases (delete before TLD)
		api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s", tldName, launchName))
		api.DELETE(fmt.Sprintf("/tlds/%s/phases/%s", tldName, gaName))

		// Premium list
		api.DELETE(fmt.Sprintf("/premium/lists/%s", plName))

		// TLD
		api.DELETE(fmt.Sprintf("/tlds/%s", tldName))

		// Registry Operator
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID))
	})
})
