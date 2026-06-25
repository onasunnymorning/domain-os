package commands

import "time"

// TLDResult is the result converting an entity TLD to a command TLDResult
type TLDResult struct {
	Name      string
	Type      string
	UName     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateTLDCommand is the command to create a TLD
type CreateTLDCommand struct {
	Name                     string
	RyID                     string
	CreateOperatorRegistrars bool // default true; creates 9998-{tld} and 9999-{tld} registrar accounts
	AllowEscrowImport        bool // default true; permits escrow imports for this TLD
}

// CreateTLDCommandResult is the result of the CreateTLDCommand
type CreateTLDCommandResult struct {
	Result TLDResult
}
