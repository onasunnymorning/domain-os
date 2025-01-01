package activities

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

type GetDomainHostsTestSuite struct {
	suite.Suite
	originalTransport http.RoundTripper
	mockTransport     *MockRoundTripper
}

func (suite *GetDomainHostsTestSuite) SetupTest() {
	// Save the original transport and replace it with a mock
	suite.originalTransport = http.DefaultTransport
	suite.mockTransport = &MockRoundTripper{}
	http.DefaultTransport = suite.mockTransport
}

func (suite *GetDomainHostsTestSuite) TearDownTest() {
	// Restore the original transport
	http.DefaultTransport = suite.originalTransport
}
func (suite *GetDomainHostsTestSuite) TestGetDomainHosts_Success() {
	body := `{
    "RoID": "1872789380826112000_DOM-APEX",
    "Name": "space.bar",
    "OriginalName": "",
    "UName": "",
    "RegistrantID": "H5033300",
    "AdminID": "H5033300",
    "TechID": "H5033300",
    "BillingID": "H5033300",
    "ClID": "9999.bar",
    "CrRr": "9999.bar",
    "UpRr": "9999.bar",
    "TLDName": "bar",
    "ExpiryDate": "2024-05-14T23:59:59Z",
    "DropCatch": false,
    "RenewedYears": 9,
    "AuthInfo": "escr0W1mP*rt",
    "CreatedAt": "2014-05-14T14:59:14Z",
    "UpdatedAt": "2024-12-27T23:48:48.156838Z",
    "Status": {
        "OK": false,
        "Inactive": false,
        "ClientTransferProhibited": false,
        "ClientUpdateProhibited": false,
        "ClientDeleteProhibited": false,
        "ClientRenewProhibited": false,
        "ClientHold": false,
        "ServerTransferProhibited": true,
        "ServerUpdateProhibited": true,
        "ServerDeleteProhibited": true,
        "ServerPenewProhibited": true,
        "ServerHold": false,
        "PendingCreate": false,
        "PendingRenew": false,
        "PendingTransfer": false,
        "PendingUpdate": false,
        "PendingRestore": false,
        "PendingDelete": false
    },
    "RGPStatus": {
        "addPeriodEnd": "0001-01-01T00:00:00Z",
        "renewPeriodEnd": "0001-01-01T00:00:00Z",
        "autoRenewPeriodEnd": "0001-01-01T00:00:00Z",
        "transferLockPeriodEnd": "0001-01-01T00:00:00Z",
        "redemptionPeriodEnd": "0001-01-01T00:00:00Z",
        "purgeDate": "0001-01-01T00:00:00Z"
    },
    "GrandFathering": {
        "Amount": 0,
        "Currency": "",
        "ExpiryCondition": "",
        "VoidDate": null
    },
    "Hosts": [
        {
            "RoID": "1872788350793129984_HOST-APEX",
            "Name": "ns.rackspace.com",
            "Addresses": [],
            "ClID": "9999.bar",
            "CrRr": "9999.bar",
            "UpRr": "",
            "CrDate": "2024-12-27T23:35:12.239509Z",
            "UpDate": "2024-12-27T23:53:05.233652Z",
            "InBailiwick": false,
            "Status": {
                "OK": true,
                "Linked": true,
                "PendingCreate": false,
                "PendingDelete": false,
                "PendingUpdate": false,
                "PendingTransfer": false,
                "ClientDeleteProhibited": false,
                "ClientUpdateProhibited": false,
                "ServerDeleteProhibited": false,
                "ServerUpdateProhibited": false
            }
        },
        {
            "RoID": "1872788350918959105_HOST-APEX",
            "Name": "ns2.rackspace.com",
            "Addresses": [],
            "ClID": "9999.bar",
            "CrRr": "9999.bar",
            "UpRr": "",
            "CrDate": "2024-12-27T23:35:12.278825Z",
            "UpDate": "2024-12-27T23:53:05.258693Z",
            "InBailiwick": false,
            "Status": {
                "OK": true,
                "Linked": true,
                "PendingCreate": false,
                "PendingDelete": false,
                "PendingUpdate": false,
                "PendingTransfer": false,
                "ClientDeleteProhibited": false,
                "ClientUpdateProhibited": false,
                "ServerDeleteProhibited": false,
                "ServerUpdateProhibited": false
            }
        }
    ]
}`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	result, err := GetDomain("space.bar")
	suite.NoError(err, "Expected no error for successful domain retrieval")
	suite.NotNil(result, "Expected a valid response")
	suite.Equal("space.bar", result.Name.String(), "Expected domain name to match")
	suite.Equal(len(result.Hosts), 2, "Expected two hosts")
}

func (suite *GetDomainHostsTestSuite) TestGetDomainHosts_BadRequest() {
	body := `Bad Request`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	result, err := GetDomain("example.com")
	suite.Error(err, "Expected an error for bad request")
	suite.Nil(result, "Expected no result for bad request")
	suite.Contains(err.Error(), "400", "Error should include HTTP status code")
	suite.Contains(err.Error(), "Bad Request", "Error should include response body")
}

func (suite *GetDomainHostsTestSuite) TestGetDomainHosts_ParseError() {
	body := `invalid json`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	result, err := GetDomain("example.com")
	suite.Error(err, "Expected an error for invalid JSON response")
	suite.Nil(result, "Expected no result for invalid JSON")
	suite.Contains(err.Error(), "failed to unmarshal response body", "Error should indicate parse failure")
}

func TestGetDomainHostsTestSuite(t *testing.T) {
	suite.Run(t, new(GetDomainHostsTestSuite))
}
