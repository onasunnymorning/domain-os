package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDomain_TableName(t *testing.T) {
	d := Domain{}
	require.Equal(t, "domains", d.TableName())
}

func getValidDBDomain() *Domain {
	return &Domain{
		RoID:         123456,
		Name:         "example.com",
		OriginalName: "example.com",
		UName:        "example.com",
		RegistrantID: "123456",
		AdminID:      "123456",
		TechID:       "123456",
		BillingID:    "123456",
		ClID:         "123456",
		CrRr:         "123456",
		UpRr:         "123456",
		TLDName:      "com",
		RenewedYears: 1,
		AuthInfo:     "abc123",
	}
}

func TestDomain_ToDomain(t *testing.T) {
	dbDomain := getValidDBDomain()
	d := ToDomain(dbDomain)
	roid, _ := d.RoID.Int64()
	require.Equal(t, dbDomain.RoID, roid)
	require.Equal(t, dbDomain.Name, d.Name.String())
	require.Equal(t, dbDomain.OriginalName, d.OriginalName)
	require.Equal(t, dbDomain.UName, d.UName)
	require.Equal(t, dbDomain.RegistrantID, d.RegistrantID.String())
	require.Equal(t, dbDomain.AdminID, d.AdminID.String())
	require.Equal(t, dbDomain.TechID, d.TechID.String())
	require.Equal(t, dbDomain.BillingID, d.BillingID.String())
	require.Equal(t, dbDomain.ClID, d.ClID.String())
	require.Equal(t, dbDomain.CrRr, d.CrRr.String())
	require.Equal(t, dbDomain.UpRr, d.UpRr.String())
	require.Equal(t, dbDomain.TLDName, d.TLDName.String())
	require.Equal(t, dbDomain.RenewedYears, d.RenewedYears)
	require.Equal(t, dbDomain.AuthInfo, d.AuthInfo.String())
}

func TestDomain_ToDBDomain(t *testing.T) {
	dbDom := getValidDBDomain()
	d := ToDomain(dbDom)
	dbDomain := ToDBDomain(d)

	require.Equal(t, dbDom.RoID, dbDomain.RoID)
	require.Equal(t, dbDom.Name, dbDomain.Name)
	require.Equal(t, dbDom.OriginalName, dbDomain.OriginalName)
	require.Equal(t, dbDom.UName, dbDomain.UName)
	require.Equal(t, dbDom.RegistrantID, dbDomain.RegistrantID)
	require.Equal(t, dbDom.AdminID, dbDomain.AdminID)
	require.Equal(t, dbDom.TechID, dbDomain.TechID)
	require.Equal(t, dbDom.BillingID, dbDomain.BillingID)
	require.Equal(t, dbDom.ClID, dbDomain.ClID)
	require.Equal(t, dbDom.CrRr, dbDomain.CrRr)
	require.Equal(t, dbDom.UpRr, dbDomain.UpRr)
	require.Equal(t, dbDom.TLDName, dbDomain.TLDName)
	require.Equal(t, dbDom.RenewedYears, dbDomain.RenewedYears)
	require.Equal(t, dbDom.AuthInfo, dbDomain.AuthInfo)
	require.Equal(t, dbDom.CreatedAt, dbDomain.CreatedAt)
	require.Equal(t, dbDom.UpdatedAt, dbDomain.UpdatedAt)
	require.Equal(t, dbDom.DomainStatus, dbDomain.DomainStatus)
	require.Equal(t, dbDom.DomainsRGPStatus, dbDomain.DomainsRGPStatus)

}
