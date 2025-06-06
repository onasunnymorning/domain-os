package postgres

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func TestDomain_TableName(t *testing.T) {
	d := Domain{}
	require.Equal(t, "domains", d.TableName())
}

func getValidDBDomain() *Domain {
	t := time.Now().AddDate(1, 0, 0)
	contactID := "123456"
	rarClid := "domtestRar"
	return &Domain{
		RoID:         12345678,
		Name:         "example.domaintesttld",
		OriginalName: "example.domaintesttld",
		UName:        "example.domaintesttld",
		RegistrantID: &contactID,
		AdminID:      &contactID,
		TechID:       &contactID,
		BillingID:    &contactID,
		ClID:         rarClid,
		CrRr:         &rarClid,
		UpRr:         &rarClid,
		TLDName:      "domaintesttld",
		RenewedYears: 1,
		AuthInfo:     "abc123",
		Hosts: []Host{
			{
				Name: "ns1.example.com",
			},
			{
				Name: "ns2.example.com",
			},
		},
		DomainGrandFathering: entities.DomainGrandFathering{
			GFAmount:          100,
			GFCurrency:        "USD",
			GFExpiryCondition: "transfer",
			GFVoidDate:        &t,
		},
	}
}

func TestDomain_ToDomain(t *testing.T) {
	dbDomain := getValidDBDomain()
	d := ToDomain(dbDomain)
	roid, _ := d.RoID.Int64()
	require.Equal(t, dbDomain.RoID, roid)
	require.Equal(t, dbDomain.Name, d.Name.String())
	require.Equal(t, dbDomain.OriginalName, d.OriginalName.String())
	require.Equal(t, dbDomain.UName, d.UName.String())
	require.Equal(t, *dbDomain.RegistrantID, d.RegistrantID.String())
	require.Equal(t, *dbDomain.AdminID, d.AdminID.String())
	require.Equal(t, *dbDomain.TechID, d.TechID.String())
	require.Equal(t, *dbDomain.BillingID, d.BillingID.String())
	require.Equal(t, dbDomain.ClID, d.ClID.String())
	require.Equal(t, *dbDomain.CrRr, d.CrRr.String())
	require.Equal(t, *dbDomain.UpRr, d.UpRr.String())
	require.Equal(t, dbDomain.TLDName, d.TLDName.String())
	require.Equal(t, dbDomain.RenewedYears, d.RenewedYears)
	require.Equal(t, dbDomain.AuthInfo, d.AuthInfo.String())
	require.Equal(t, len(dbDomain.Hosts), len(d.Hosts))
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
	require.Equal(t, dbDom.DomainRGPStatus, dbDomain.DomainRGPStatus)
	require.Equal(t, len(dbDom.Hosts), len(dbDomain.Hosts))

}
