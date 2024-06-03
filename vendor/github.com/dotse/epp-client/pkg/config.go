// Copyright (c) 2022 The Swedish Internet Foundation
//
// Distributed under the MIT License. (See accompanying LICENSE file or copy at
// <https://opensource.org/licenses/MIT>.)

package pkg

import (
	"time"

	"github.com/guregu/null"
)

// CommandConfig configuration of default data or the different commands.
var CommandConfig = []*Command{
	{
		Name: "contact",
		SubCommand: []*Command{
			{
				Name: "check",
				DefaultData: &ContactCheck{
					ContactIDs: []string{
						"contact-1",
					},
					ClTrID: "ABC-123",
				},
				Template: "contact_check.xml",
			}, {
				Name: "create",
				DefaultData: &ContactCreate{
					ID: "contact-1",
					PostalInfo: []*PostalInfo{{
						Location:     "loc",
						Name:         "My Office",
						Organization: "My Org",
						Address: &PostalAddress{
							Street: []string{
								"Drottninggatan",
							},
							City:        "Stockholm",
							PostalCode:  "12345",
							CountryCode: "SE",
						},
					}},
					Voice: &VoiceFax{
						X:      "46",
						Number: "+46.070123456",
					},
					Email: "abc@def.se",
					Disclose: &Disclose{
						Disclose:  null.BoolFrom(true).Ptr(),
						LocalName: true,
						Fax:       true,
					},
					OrganisationNumber:  "[SE]802405-0190",
					ValueAddedTaxNumber: "SE802405019001",
					ClTrID:              "ABC-123",
				},
				Template: "contact_create.xml",
			}, {
				Name: "delete",
				DefaultData: &ContactDelete{
					ID:     "contact-1",
					ClTrID: "ABC-123",
				},
				Template: "contact_delete.xml",
			}, {
				Name: "info",
				DefaultData: &ContactInfo{
					ID:     "contact-1",
					ClTrID: "ABC-123",
				},
				Template: "contact_info.xml",
			}, {
				Name: "update",
				DefaultData: &ContactUpdate{
					ID:     "contact-1",
					ClTrID: "ABC-123",
				},
				Template: "contact_update.xml",
			},
		},
	},
	{
		Name: "domain",
		SubCommand: []*Command{
			{
				Name: "check",
				DefaultData: &DomainCheck{
					DomainNames: []string{
						"domain1.se",
						"domain2.se",
					},
					ClTrID: "ABC-1234",
				},
				Template: "domain_check.xml",
			}, {
				Name: "create",
				DefaultData: &DomainCreate{
					Name: "domain1.se",
					Period: &Period{
						Unit: "m",
						Time: 12,
					},
					Hosts: []string{
						"host1.ns",
					},
					Registrant: "contact-1",
					ClTrID:     "ABC-123",
				},
				Template: "domain_create.xml",
			}, {
				Name: "delete",
				DefaultData: &DomainDelete{
					Name:   "domain1.se",
					ClTrID: "ABC-123",
				},
				Template: "domain_delete.xml",
			}, {
				Name: "info",
				DefaultData: &DomainInfo{
					Name:   "domain1.se",
					ClTrID: "ABC-123",
				},
				Template: "domain_info.xml",
			}, {
				Name: "renew",
				DefaultData: &DomainRenew{
					Name:                  "domain1.se",
					CurrentExpirationDate: time.Now().AddDate(0, 1, 0).Format("2006-01-02"),
					ClTrID:                "ABC-123",
				},
				Template: "domain_renew.xml",
			}, {
				Name: "transfer",
				DefaultData: &DomainTransfer{
					Name:     "domain1.se",
					Password: "123456pw!",
					ClTrID:   "ABC-123",
				},
				Template: "domain_transfer.xml",
			}, {
				Name: "update",
				DefaultData: &DomainUpdate{
					Name:   "domain1.se",
					ClTrID: "ABC-123",
				},
				Template: "domain_update.xml",
			},
		},
	},
	{
		Name: "host",
		SubCommand: []*Command{
			{
				Name: "check",
				DefaultData: &HostCheck{
					HostNames: []string{
						"host1.ns",
					},
					ClTrID: "ABC-123",
				},
				Template: "host_check.xml",
			}, {
				Name: "create",
				DefaultData: &HostCreate{
					Name: "host1.ns",
					Addresses: []*HostAddress{{
						IP:      "192.168.1.2",
						Version: "v4",
					}},
					ClTrID: "ABC-123",
				},
				Template: "host_create.xml",
			}, {
				Name: "delete",
				DefaultData: &HostDelete{
					Name:   "host1.ns",
					ClTrID: "ABC-123",
				},
				Template: "host_delete.xml",
			}, {
				Name: "info",
				DefaultData: &HostInfo{
					Name:   "host1.ns",
					ClTrID: "ABC-123",
				},
				Template: "host_info.xml",
			}, {
				Name: "update",
				DefaultData: &HostUpdate{
					Name:   "host1.ns",
					ClTrID: "ABC-123",
				},
				Template: "host_update.xml",
			},
		},
	},
	{
		Name: "login",
		DefaultData: &LoginData{
			Username: "epp100000",
			Password: "apa",
			Namespaces: []string{
				"urn:ietf:params:xml:ns:host-1.0",
				"urn:ietf:params:xml:ns:contact-1.0",
				"urn:ietf:params:xml:ns:domain-1.0",
			},
			ExtensionNamespaces: []string{
				"urn:ietf:params:xml:ns:secDNS-1.0",
				"urn:ietf:params:xml:ns:secDNS-1.1",
				"urn:se:iis:xml:epp:iis-1.2",
				"urn:se:iis:xml:epp:registryLock-1.0",
			},
			Version: "1.0",
			Lang:    "en",
			ClTrID:  "ABC-12345",
		},
		Template: "login.xml",
	},
	{
		Name:     "hello",
		Template: "hello.xml",
	},
	{
		Name:        "logout",
		DefaultData: &LogoutData{ClTrID: "ABC-1234"},
		Template:    "logout.xml",
	},
	{
		Name: "poll",
		DefaultData: &Poll{
			Operation: "ack",
			MessageID: "1",
			ClTrID:    "ABC-123",
		},
		Template: "poll.xml",
	},
}
