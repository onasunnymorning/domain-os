package tests

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// api is the shared test harness available to all test files in this package.
var api *TestAPI

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Integration Tests Suite")
}

var _ = BeforeSuite(func() {
	var err error
	api, err = NewTestAPI()
	Expect(err).NotTo(HaveOccurred(), "Failed to initialize test API harness")
})

var _ = AfterSuite(func() {
	if api != nil && api.Server != nil {
		api.Server.Close()
	}
})
