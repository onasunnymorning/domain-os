package tests

import (
	"fmt"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HostController", Ordered, func() {
	const registrarClid = "hostTestRar"

	hostCmd := &commands.CreateHostCommand{
		Name:      "ns1.example.com",
		Addresses: []string{"192.0.2.1"},
		ClID:      entities.ClIDType(registrarClid),
		CrRr:      entities.ClIDType(registrarClid),
	}

	var createdHost entities.Host

	BeforeAll(func() {
		registrarPayload := testRegistrar(registrarClid, "Host Test Registrar", 10003)
		resp := api.POST("/registrars", registrarPayload)
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	AfterAll(func() {
		api.DELETE(fmt.Sprintf("/registrars/%s", registrarClid))
	})

	It("should create a new host", func() {
		resp := api.POST("/hosts", hostCmd)
		Expect(resp.Code).To(Equal(http.StatusCreated))

		err := DecodeJSON(resp, &createdHost)
		Expect(err).NotTo(HaveOccurred())
		Expect(createdHost.RoID.String()).NotTo(BeEmpty())
		Expect(createdHost.Name.String()).To(Equal(hostCmd.Name))
		Expect(createdHost.ClID).To(Equal(hostCmd.ClID))
	})

	It("should get the host by RoID", func() {
		resp := api.GET(fmt.Sprintf("/hosts/%s", createdHost.RoID))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var host entities.Host
		err := DecodeJSON(resp, &host)
		Expect(err).NotTo(HaveOccurred())
		Expect(host.Name.String()).To(Equal(hostCmd.Name))
		Expect(host.ClID).To(Equal(hostCmd.ClID))
	})

	It("should list hosts", func() {
		resp := api.GET("/hosts")
		Expect(resp.Code).To(Equal(http.StatusOK))

		var res map[string]interface{}
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())

		items, ok := res["Data"].([]interface{})
		Expect(ok).To(BeTrue())
		Expect(items).NotTo(BeEmpty())
	})

	It("should count hosts", func() {
		resp := api.GET("/hosts/count")
		Expect(resp.Code).To(Equal(http.StatusOK))

		var res response.CountResult
		err := DecodeJSON(resp, &res)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Count).To(BeNumerically(">", 0))
	})

	It("should add an IP address to the host", func() {
		resp := api.POSTNoBody(fmt.Sprintf("/hosts/%s/addresses/192.0.2.2", createdHost.RoID))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var host entities.Host
		err := DecodeJSON(resp, &host)
		Expect(err).NotTo(HaveOccurred())
		Expect(host.Addresses).To(HaveLen(2))
	})

	It("should remove an IP address from the host", func() {
		resp := api.DELETE(fmt.Sprintf("/hosts/%s/addresses/192.0.2.2", createdHost.RoID))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var host entities.Host
		err := DecodeJSON(resp, &host)
		Expect(err).NotTo(HaveOccurred())
		Expect(host.Addresses).To(HaveLen(1))
	})

	It("should get the host by name and clid", func() {
		resp := api.GET(fmt.Sprintf("/hostname/%s/%s", hostCmd.Name, registrarClid))
		Expect(resp.Code).To(Equal(http.StatusOK))

		var host entities.Host
		err := DecodeJSON(resp, &host)
		Expect(err).NotTo(HaveOccurred())
		Expect(host.Name.String()).To(Equal(hostCmd.Name))
		Expect(host.ClID).To(Equal(hostCmd.ClID))
	})

	It("should delete the host by RoID", func() {
		resp := api.DELETE(fmt.Sprintf("/hosts/%s", createdHost.RoID))
		Expect(resp.Code).To(Equal(http.StatusNoContent))
	})

	It("should return 404 for the deleted host", func() {
		resp := api.GET(fmt.Sprintf("/hosts/%s", createdHost.RoID))
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})
})
