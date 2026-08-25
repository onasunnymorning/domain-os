package tests

import (
	"fmt"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Registry Operator Controller Integration Test
//
// Story: manage Registry Operators through their full CRUD lifecycle
//
//	create → list → count → get → update → delete → verify deletion
var _ = Describe("RegistryOperatorController", Ordered, func() {
	const (
		ryID1 = "roTest1"
		ryID2 = "roTest2"
	)

	// ================================================================== //
	//  List — zero results (before any creates)                           //
	// ================================================================== //

	It("should list registry operators (initially may be empty)", func() {
		resp := api.GET("/registry-operators")
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListRegistryOperatorsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())
	})

	// ================================================================== //
	//  Create — unhappy paths                                             //
	// ================================================================== //

	It("should fail to create a registry operator with no body", func() {
		resp := api.POSTNoBody("/registry-operators")
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("should fail to create a registry operator with missing name", func() {
		cmd := map[string]interface{}{
			"RyID":  "noName",
			"Email": "test@test.com",
		}
		resp := api.POST("/registry-operators", cmd)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	// ================================================================== //
	//  Create — happy paths                                               //
	// ================================================================== //

	It("should create registry operator 1", func() {
		cmd := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID1,
			Name:  "RO Test One",
			Email: "admin@rotest1.com",
		}
		resp := api.POST("/registry-operators", cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create RyOp 1 failed: %s", resp.Body.String())

		var ry entities.RegistryOperator
		Expect(DecodeJSON(resp, &ry)).To(Succeed())
		Expect(ry.RyID.String()).To(Equal(ryID1))
		Expect(ry.Name).To(Equal("RO Test One"))
	})

	It("should count at least 1 registry operator", func() {
		resp := api.GET("/registry-operators/count")
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result response.CountResult
		Expect(DecodeJSON(resp, &result)).To(Succeed())
		Expect(result.Count).To(BeNumerically(">=", 1))
	})

	It("should create registry operator 2", func() {
		cmd := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID2,
			Name:  "RO Test Two",
			Email: "admin@rotest2.com",
		}
		resp := api.POST("/registry-operators", cmd)
		Expect(resp.Code).To(Equal(http.StatusCreated), "Create RyOp 2 failed: %s", resp.Body.String())

		var ry entities.RegistryOperator
		Expect(DecodeJSON(resp, &ry)).To(Succeed())
		Expect(ry.RyID.String()).To(Equal(ryID2))
	})

	It("should count at least 2 registry operators", func() {
		resp := api.GET("/registry-operators/count")
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result response.CountResult
		Expect(DecodeJSON(resp, &result)).To(Succeed())
		Expect(result.Count).To(BeNumerically(">=", 2))
	})

	// ================================================================== //
	//  List — multiple results                                            //
	// ================================================================== //

	It("should list at least 2 registry operators", func() {
		resp := api.GET("/registry-operators")
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListRegistryOperatorsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(BeNumerically(">=", 2))
	})

	// ================================================================== //
	//  Get — happy path                                                   //
	// ================================================================== //

	It("should get registry operator 1 by ID", func() {
		resp := api.GET(fmt.Sprintf("/registry-operators/%s", ryID1))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var ry entities.RegistryOperator
		Expect(DecodeJSON(resp, &ry)).To(Succeed())
		Expect(ry.RyID.String()).To(Equal(ryID1))
		Expect(ry.Name).To(Equal("RO Test One"))
	})

	It("should return 404 for non-existent registry operator", func() {
		resp := api.GET("/registry-operators/nonexistent")
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	// ================================================================== //
	//  Update — happy path                                                //
	// ================================================================== //

	It("should update registry operator 1", func() {
		updateBody := &entities.RegistryOperator{
			RyID:  entities.ClIDType(ryID1),
			Name:  "RO Test One Updated",
			Email: "updated@rotest1.com",
		}
		resp := api.PUT(fmt.Sprintf("/registry-operators/%s", ryID1), updateBody)
		Expect(resp.Code).To(Equal(http.StatusOK), "Update RyOp failed: %s", resp.Body.String())

		var ry entities.RegistryOperator
		Expect(DecodeJSON(resp, &ry)).To(Succeed())
		Expect(ry.Name).To(Equal("RO Test One Updated"))
	})

	It("should verify registry operator 1 was updated", func() {
		resp := api.GET(fmt.Sprintf("/registry-operators/%s", ryID1))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var ry entities.RegistryOperator
		Expect(DecodeJSON(resp, &ry)).To(Succeed())
		Expect(ry.Name).To(Equal("RO Test One Updated"))
		Expect(ry.Email).To(Equal("updated@rotest1.com"))
	})

	// ================================================================== //
	//  Delete — happy and unhappy paths                                   //
	// ================================================================== //

	It("should delete registry operator 1", func() {
		resp := api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID1))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should handle delete of already-deleted registry operator 1", func() {
		resp := api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID1))
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusNoContent),
			Equal(http.StatusNotFound),
			Equal(http.StatusInternalServerError),
		))
	})

	It("should return 404 for deleted registry operator 1", func() {
		resp := api.GET(fmt.Sprintf("/registry-operators/%s", ryID1))
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	It("should delete registry operator 2", func() {
		resp := api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID2))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should return 404 for deleted registry operator 2", func() {
		resp := api.GET(fmt.Sprintf("/registry-operators/%s", ryID2))
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})
})
