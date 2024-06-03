// Copyright (c) 2022 The Swedish Internet Foundation
//
// Distributed under the MIT License. (See accompanying LICENSE file or copy at
// <https://opensource.org/licenses/MIT>.)

package pkg

// Command holds all information about a command to the server.
type Command struct {
	Name        string
	SubCommand  []*Command
	DefaultData any
	Template    string
}

// IPAddress the needed information about an IP address.
type IPAddress struct {
	Name    string `xml:"name"`
	Address string `xml:"ip"`
}

// LoginData the available data for a login command.
type LoginData struct {
	Username            string   `xml:"clID"`
	Password            string   `xml:"pw"`
	Namespaces          []string `xml:"objURI"`
	ExtensionNamespaces []string `xml:"extURI"`
	Version             string   `xml:"version"`
	Lang                string   `xml:"lang"`
	ClTrID              string   `xml:"clTrID"`
}

// LogoutData the available data for a logout command.
type LogoutData struct {
	ClTrID string `xml:"clTrID"`
}

// DomainCheck the available data for a domain check command.
type DomainCheck struct {
	DomainNames []string `xml:"name"`
	ClTrID      string   `xml:"clTrID"`
}

// Period the data needed for a period element.
type Period struct {
	Unit string `xml:"unit"`
	Time int    `xml:"period"`
}

// DsData information for a dnssec element.
type DsData struct {
	KeyTag     int    `xml:"keyTag"`
	Algorithm  int    `xml:"alg"`
	DigestType int    `xml:"digestType"`
	Digest     string `xml:"digest"`
}

// DomainCreate the available data for a domain create command.
type DomainCreate struct {
	Name           string    `xml:"name"`
	Period         *Period   `xml:"period"`
	Hosts          []string  `xml:"hostObj"`
	HostAttributes []string  `xml:"hostName"`
	Password       string    `xml:"pw"`
	Registrant     string    `xml:"registrant"`
	DsData         []*DsData `xml:"dsData"`
	RegistryLock   string    `xml:"unlock"`
	ClTrID         string    `xml:"clTrID"`
}

// DomainDelete the available data for a domain delete command.
type DomainDelete struct {
	Name   string `xml:"name"`
	ClTrID string `xml:"clTrID"`
}

// DomainInfo the available data for a domain info command.
type DomainInfo struct {
	Name   string `xml:"name"`
	Hosts  string `xml:"hosts"`
	ClTrID string `xml:"clTrID"`
}

// DomainRenew the available data for a domain renew command.
type DomainRenew struct {
	Name                  string  `xml:"name"`
	CurrentExpirationDate string  `xml:"currExpDate"`
	Period                *Period `xml:"period"`
	ClTrID                string  `xml:"clTrID"`
}

// DomainTransfer the available data for a domain transfer command.
type DomainTransfer struct {
	Name     string   `xml:"name"`
	Password string   `xml:"pw"`
	Hosts    []string `xml:"hostObj"`
	ClTrID   string   `xml:"clTrID"`
}

// Status hold information for the status element.
type Status struct {
	Status   string `xml:"s"`
	Language string `xml:"lang"`
	Message  string `xml:"message"`
}

// DomainUpdateAction hold information for the domain add and rem elements.
type DomainUpdateAction struct {
	Hosts          []string  `xml:"hostObj"`
	HostAttributes []string  `xml:"hostAttr"`
	Statuses       []*Status `xml:"status"`
}

// DomainUpdateChange hold information for the domain chg element.
type DomainUpdateChange struct {
	Registrant string `xml:"registrant"`
	Password   string `xml:"pw"`
}

// DomainUpdate the available data for a domain update command.
type DomainUpdate struct {
	Name                  string              `xml:"name"`
	Add                   *DomainUpdateAction `xml:"add"`
	Remove                *DomainUpdateAction `xml:"rem"`
	Change                *DomainUpdateChange `xml:"chg"`
	DNSSecAdd             []*DsData           `xml:"secDNS:add"`
	DNSSecRemove          []*DsData           `xml:"secDNS:rem"`
	DNSSecRemoveAll       bool                `xml:"secDNS:rem:all"`
	RegistryLock          string              `xml:"unlock"`
	ClientDelete          *bool               `xml:"clientDelete"`
	ClientDeleteAtExpDate bool                `xml:"clientDelete:atExpDate"`
	ClTrID                string              `xml:"clTrID"`
}

// HostCheck the available data for a host check command.
type HostCheck struct {
	HostNames []string `xml:"name"`
	ClTrID    string   `xml:"clTrID"`
}

// HostAddress hold information about the host address element.
type HostAddress struct {
	IP      string `xml:"addr"`
	Version string `xml:"version"`
}

// HostCreate the available data for a host create command.
type HostCreate struct {
	Name      string         `xml:"name"`
	Addresses []*HostAddress `xml:"addr"`
	ClTrID    string         `xml:"clTrID"`
}

// HostDelete the available data for a host delete command.
type HostDelete struct {
	Name   string `xml:"name"`
	ClTrID string `xml:"clTrID"`
}

// HostInfo the available data for a host info command.
type HostInfo struct {
	Name   string `xml:"name"`
	ClTrID string `xml:"clTrID"`
}

// HostUpdate the available data for a host update command.
type HostUpdate struct {
	Name            string         `xml:"name"`
	AddAddresses    []*HostAddress `xml:"add"`
	RemoveAddresses []*HostAddress `xml:"rem"`
	ChangeAddresses []*HostAddress `xml:"chg"`
	ClTrID          string         `xml:"clTrID"`
}

// Poll the available data for a poll command.
type Poll struct {
	Operation string `xml:"op"`
	MessageID string `xml:"msgID"`
	ClTrID    string `xml:"clTrID"`
}

// ContactCheck the available data for a contact check command.
type ContactCheck struct {
	ContactIDs []string `xml:"id"`
	ClTrID     string   `xml:"clTrID"`
}

// PostalAddress hold information for the addr element.
type PostalAddress struct {
	Street          []string `xml:"street"`
	City            string   `xml:"city"`
	StateOrProvince string   `xml:"sp"`
	PostalCode      string   `xml:"pc"`
	CountryCode     string   `xml:"cc"`
}

// PostalInfo hold information for the postalInfo element.
type PostalInfo struct {
	Location     string         `xml:"loc"`
	Name         string         `xml:"name"`
	Organization string         `xml:"org"`
	Address      *PostalAddress `xml:"addr"`
}

// VoiceFax hold data for the voice and fax elements.
type VoiceFax struct {
	X      string `xml:"x"`
	Number string `xml:"number"`
}

// Disclose hold information about which fields should be disclosed.
type Disclose struct {
	Disclose          *bool `xml:"disclose"`
	InternationalName bool  `xml:"name:int"`
	LocalName         bool  `xml:"name:loc"`
	InternationalOrg  bool  `xml:"org:int"`
	LocalOrg          bool  `xml:"org:loc"`
	InternationalAddr bool  `xml:"addr:int"`
	LocalAddr         bool  `xml:"addr:loc"`
	Voice             bool  `xml:"voice"`
	Fax               bool  `xml:"fax"`
	Email             bool  `xml:"email"`
}

// ContactCreate the available data for a contact create command.
type ContactCreate struct {
	ID                  string        `xml:"id"`
	PostalInfo          []*PostalInfo `xml:"postalInfo"`
	Voice               *VoiceFax     `xml:"voice"`
	Fax                 *VoiceFax     `xml:"fax"`
	Email               string        `xml:"email"`
	Disclose            *Disclose     `xml:"disclose"`
	OrganisationNumber  string        `xml:"orgno"`
	ValueAddedTaxNumber string        `xml:"vatno"`
	ClTrID              string        `xml:"clTrID"`
}

// ContactDelete the available data for a contact delete command.
type ContactDelete struct {
	ID     string `xml:"id"`
	ClTrID string `xml:"clTrID"`
}

// ContactInfo the available data for a contact info command.
type ContactInfo struct {
	ID     string `xml:"id"`
	ClTrID string `xml:"clTrID"`
	Pw     string `xml:"pw"`
	ROID   string `xml:"roid"`
}

// ContactUpdate the available data for a contact update command.
type ContactUpdate struct {
	ID         string        `xml:"id"`
	PostalInfo []*PostalInfo `xml:"postalInfo"`
	Voice      *VoiceFax     `xml:"voice"`
	Fax        *VoiceFax     `xml:"fax"`
	Email      string        `xml:"email"`
	Disclose   *Disclose     `xml:"disclose"`
	ClTrID     string        `xml:"clTrID"`
}
