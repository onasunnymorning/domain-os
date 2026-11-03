package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/onasunnymorning/domain-os/internal/interface/rest"

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
//
// policyReq performs a lifecycle policy request with the registry:policy:admin
// permission granted (TestAuthScopesHeader).
func policyReq(method, path string, body interface{}) *httptest.ResponseRecorder {
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	req, err := http.NewRequest(method, api.Server.URL+path, bytes.NewReader(payload))
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(TestAuthScopesHeader, rest.ScopeRegistryPolicyAdmin)
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	return toRecorder(resp)
}

var _ = Describe("PhaseController", Ordered, func() {
	const (
		ryID       = "phsTestRyOp"
		otherRyID  = "phsOtherRyOp"
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
		resp = api.POST("/registry-operators", &commands.CreateRegistryOperatorCommand{RyID: otherRyID, Name: "Another operator", Email: "other@phstest.com"})
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

	It("should create a launch phase with its launch settings", func() {
		cmd := &commands.CreatePhaseCommand{
			Name:   "PhsLandrush",
			Type:   "Launch",
			Starts: time.Now().UTC().Add(30 * 24 * time.Hour),
			Launch: &entities.LaunchSettings{Mode: entities.LaunchModeLandrush, Model: entities.RegistrationModelApplication, AcceptSignedMarks: true},
		}
		resp := api.POST(fmt.Sprintf("/tlds/%s/phases", tldName), cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
		var phase entities.Phase
		Expect(DecodeJSON(resp, &phase)).To(Succeed())
		Expect(phase.Launch.Mode).To(Equal(entities.LaunchModeLandrush))
		Expect(phase.Launch.Model).To(Equal(entities.RegistrationModelApplication))

		// Launch settings on a GA phase are refused.
		cmd = &commands.CreatePhaseCommand{Name: "PhsGAWithLaunch", Type: "GA", Starts: time.Now().UTC().Add(400 * 24 * time.Hour),
			Launch: &entities.LaunchSettings{Mode: entities.LaunchModeOpen, Model: entities.RegistrationModelFirstCome}}
		resp = api.POST(fmt.Sprintf("/tlds/%s/phases", tldName), cmd)
		Expect(resp.Code).To(Equal(http.StatusBadRequest), resp.Body.String())
	})

	It("should default a launch phase to open and first come", func() {
		Expect(createdLaunchPhase.Launch.Mode).To(Equal(entities.LaunchModeOpen))
		Expect(createdLaunchPhase.Launch.Model).To(Equal(entities.RegistrationModelFirstCome))
		Expect(createdLaunchPhase.Launch.AcceptSignedMarks).To(BeTrue())
	})

	It("should set the launch settings of a launch phase", func() {
		// The future landrush phase: the model may still change.
		resp := api.PUTAs(fmt.Sprintf("/tlds/%s/phases/%s/launch", tldName, "PhsLandrush"), ryID,
			&commands.SetLaunchSettingsCommand{LaunchSettings: entities.LaunchSettings{Mode: entities.LaunchModeLandrush, Model: entities.RegistrationModelFirstCome, RetainMarks: true}})
		Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
		var phase entities.Phase
		Expect(DecodeJSON(resp, &phase)).To(Succeed())
		Expect(phase.Launch.Model).To(Equal(entities.RegistrationModelFirstCome))
		Expect(phase.Launch.RetainMarks).To(BeTrue())

		// The launch phase that started two days ago: its mode is frozen...
		resp = api.PUTAs(fmt.Sprintf("/tlds/%s/phases/%s/launch", tldName, launchName), ryID,
			&commands.SetLaunchSettingsCommand{LaunchSettings: entities.LaunchSettings{Mode: entities.LaunchModeLandrush, Model: entities.RegistrationModelFirstCome}})
		Expect(resp.Code).To(Equal(http.StatusBadRequest), resp.Body.String())
		// ...and so is its model...
		resp = api.PUTAs(fmt.Sprintf("/tlds/%s/phases/%s/launch", tldName, launchName), ryID,
			&commands.SetLaunchSettingsCommand{LaunchSettings: entities.LaunchSettings{Mode: entities.LaunchModeOpen, Model: entities.RegistrationModelApplication}})
		Expect(resp.Code).To(Equal(http.StatusBadRequest), resp.Body.String())
		// ...but its mark settings may change.
		resp = api.PUTAs(fmt.Sprintf("/tlds/%s/phases/%s/launch", tldName, launchName), ryID,
			&commands.SetLaunchSettingsCommand{LaunchSettings: entities.LaunchSettings{Mode: entities.LaunchModeOpen, Model: entities.RegistrationModelFirstCome, AcceptSignedMarks: true, AcceptCodeMarks: true}})
		Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())

		// Invalid settings and another operator's TLD.
		resp = api.PUTAs(fmt.Sprintf("/tlds/%s/phases/%s/launch", tldName, launchName), ryID,
			&commands.SetLaunchSettingsCommand{LaunchSettings: entities.LaunchSettings{Mode: "premium", Model: entities.RegistrationModelFirstCome}})
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
		resp = api.PUTAs(fmt.Sprintf("/tlds/%s/phases/%s/launch", tldName, launchName), otherRyID,
			&commands.SetLaunchSettingsCommand{LaunchSettings: entities.LaunchSettings{Mode: entities.LaunchModeOpen, Model: entities.RegistrationModelFirstCome}})
		Expect(resp.Code).To(Equal(http.StatusNotFound))
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

	It("should refuse launch settings on a GA phase", func() {
		resp := api.PUTAs(fmt.Sprintf("/tlds/%s/phases/%s/launch", tldName, gaName), ryID,
			&commands.SetLaunchSettingsCommand{LaunchSettings: entities.LaunchSettings{Mode: entities.LaunchModeOpen, Model: entities.RegistrationModelFirstCome}})
		Expect(resp.Code).To(Equal(http.StatusBadRequest), resp.Body.String())
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
		Expect(phase.LifecyclePolicyName).To(Equal(entities.LifecyclePolicyStandard), "the phase still names the standard lifecycle")
		Expect(phase.LifecyclePolicy).NotTo(BeNil(), "resolved in the response")
	})

	It("should refuse a phase policy whose label lengths are inconsistent", func() {
		policyCmd := &commands.UpdatePhasePolicyCommand{Policy: &entities.PhasePolicy{MinLabelLength: 10, MaxLabelLength: 5}}
		resp := api.PUT(fmt.Sprintf("/tlds/%s/phases/%s/policy", tldName, gaName), policyCmd)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	// ================================================================== //
	//  Lifecycle policy (ADR 0023)                                        //
	// ================================================================== //

	It("should name a lifecycle policy on the phase and resolve it", func() {
		three := 3
		// Policies are platform-wide: the registry:policy:admin permission, not a tenant.
		resp := api.PUT("/lifecycle-policies/phs-short", &commands.SaveLifecyclePolicyCommand{Description: "short redemption", Settings: entities.LifecycleSettings{RedemptionGP: &three}})
		Expect(resp.Code).To(Equal(http.StatusForbidden), "without the permission")
		resp = policyReq(http.MethodPut, "/lifecycle-policies/phs-short", &commands.SaveLifecyclePolicyCommand{Description: "short redemption", Settings: entities.LifecycleSettings{RedemptionGP: &three}})
		Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())

		// A phase edit under the TLD's operator (INV-02): without a scope it
		// is refused, and another operator finds no such TLD.
		resp = api.PUT(fmt.Sprintf("/tlds/%s/phases/%s/lifecycle-policy", tldName, gaName), &commands.SetPhaseLifecyclePolicyCommand{LifecyclePolicy: "phs-short"})
		Expect(resp.Code).To(Equal(http.StatusBadRequest), resp.Body.String())
		resp = api.PUTAs(fmt.Sprintf("/tlds/%s/phases/%s/lifecycle-policy", tldName, gaName), otherRyID, &commands.SetPhaseLifecyclePolicyCommand{LifecyclePolicy: "phs-short"})
		Expect(resp.Code).To(Equal(http.StatusNotFound), resp.Body.String())
		resp = api.GET(fmt.Sprintf("/tlds/%s/phases/%s", tldName, gaName))
		var phase entities.Phase
		Expect(DecodeJSON(resp, &phase)).To(Succeed())
		Expect(phase.LifecyclePolicyName).To(Equal(entities.LifecyclePolicyStandard), "left as it was")

		resp = api.PUTAs(fmt.Sprintf("/tlds/%s/phases/%s/lifecycle-policy", tldName, gaName), ryID, &commands.SetPhaseLifecyclePolicyCommand{LifecyclePolicy: "phs-short"})
		Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
		Expect(DecodeJSON(resp, &phase)).To(Succeed())
		Expect(phase.LifecyclePolicyName).To(Equal("phs-short"))
		Expect(phase.Lifecycle().RedemptionDays()).To(Equal(3))
		Expect(phase.Lifecycle().PendingDeleteDays()).To(Equal(entities.PendingDeleteGP))

		resp = api.GET(fmt.Sprintf("/tlds/%s/phases/%s", tldName, gaName))
		Expect(resp.Code).To(Equal(http.StatusOK))
		Expect(DecodeJSON(resp, &phase)).To(Succeed())
		Expect(phase.LifecyclePolicyName).To(Equal("phs-short"))
		Expect(phase.LifecyclePolicy).NotTo(BeNil())

		resp = api.GET("/lifecycle-policies/phs-short")
		Expect(resp.Code).To(Equal(http.StatusOK), "reading the catalogue needs no permission")
		var policy entities.LifecyclePolicy
		Expect(DecodeJSON(resp, &policy)).To(Succeed())
		Expect(policy.UsedBy).To(ContainElement(entities.LifecyclePolicyUse{TLD: tldName, Phase: gaName}))

		resp = policyReq(http.MethodDelete, "/lifecycle-policies/phs-short", nil)
		Expect(resp.Code).To(Equal(http.StatusConflict), "named by a phase")

		resp = api.PUTAs(fmt.Sprintf("/tlds/%s/phases/%s/lifecycle-policy", tldName, gaName), ryID, &commands.SetPhaseLifecyclePolicyCommand{LifecyclePolicy: "no-such-policy"})
		Expect(resp.Code).To(Equal(http.StatusNotFound))

		resp = api.PUTAs(fmt.Sprintf("/tlds/%s/phases/%s/lifecycle-policy", tldName, gaName), ryID, &commands.SetPhaseLifecyclePolicyCommand{LifecyclePolicy: entities.LifecyclePolicyStandard})
		Expect(resp.Code).To(Equal(http.StatusOK))
		resp = policyReq(http.MethodDelete, "/lifecycle-policies/phs-short", nil)
		Expect(resp.Code).To(Equal(http.StatusNoContent))
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
		resp := api.DELETEAs(fmt.Sprintf("/tlds/%s/phases/%s", tldName, gaName), ryID)
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
		api.DELETEAs(fmt.Sprintf("/tlds/%s/phases/%s", tldName, launchName), ryID)
		api.DELETEAs(fmt.Sprintf("/tlds/%s/phases/%s", tldName, "PhsLandrush"), ryID)
		api.DELETEAs(fmt.Sprintf("/tlds/%s/phases/%s", tldName, gaName), ryID)

		// Premium list
		api.DELETE(fmt.Sprintf("/premium/lists/%s", plName))

		// TLD
		api.DELETEAs(fmt.Sprintf("/tlds/%s", tldName), ryID)

		// Registry Operator
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID))
	})
})
