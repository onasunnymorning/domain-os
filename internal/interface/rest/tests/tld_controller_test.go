package tests

import (
	"fmt"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TLDController", Ordered, func() {
	const (
		tldName  = "testzone"
		ryID     = "tldTestRyOp"
		statusOp = "AllowEscrowImport"
	)

	tldReq := &request.CreateTLDRequest{
		Name: tldName,
		RyID: ryID,
	}

	BeforeAll(func() {
		ryCmd := &commands.CreateRegistryOperatorCommand{
			RyID:  ryID,
			Name:  "TLD Test Registry Operator",
			Email: "admin@tldtest.com",
		}
		resp := api.POST("/registry-operators", ryCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	AfterAll(func() {
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID))
	})

	It("should create a new TLD", func() {
		resp := api.POST("/tlds", tldReq)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		var result entities.TLD
		err := DecodeJSON(resp, &result)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Name.String()).To(Equal(tldName))
	})

	It("should fail to create a duplicate TLD", func() {
		resp := api.POST("/tlds", tldReq)
		Expect(resp.Code).To(SatisfyAny(
			Equal(http.StatusBadRequest),
			Equal(http.StatusConflict),
			Equal(http.StatusInternalServerError),
		))
	})

	It("should get the TLD by name", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s", tldName))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var tld map[string]interface{}
		err := DecodeJSON(resp, &tld)
		Expect(err).NotTo(HaveOccurred())
		Expect(tld["Name"]).To(Equal(tldName))
		Expect(tld["RyID"]).To(Equal(ryID))
	})

	It("should list TLDs", func() {
		resp := api.GET("/tlds")
		Expect(resp.Code).To(Equal(http.StatusOK))

		res := response.ListItemResult{}
		filter := queries.ListTldsFilter{}
		res.Meta.Filter = &filter
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		itemSlice, ok := res.Data.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(itemSlice)).To(BeNumerically(">", 0))
	})

	It("should count TLDs", func() {
		resp := api.GET("/tlds/count")
		Expect(resp.Code).To(Equal(http.StatusOK))

		var result response.CountResult
		err := DecodeJSON(resp, &result)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Count).To(BeNumerically(">", 0))
	})

	It("should set a status on the TLD", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/tlds/%s/status/%s", tldName, statusOp))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should remove the status from the TLD", func() {
		resp := api.DELETE(fmt.Sprintf("/tlds/%s/status/%s", tldName, statusOp))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should delete the TLD", func() {
		resp := api.DELETE(fmt.Sprintf("/tlds/%s", tldName))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should return 404 for the deleted TLD", func() {
		resp := api.GET(fmt.Sprintf("/tlds/%s", tldName))
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})
})
