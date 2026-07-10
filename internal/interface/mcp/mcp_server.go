// Package mcp provides an MCP (Model Context Protocol) server adapter for
// domain-os. It exposes read-only registry tools, allowing AI models to
// query domain state through the standard MCP protocol.
//
// The adapter supports two transports:
//   - stdio: for local use by AI IDEs (e.g. Claude Desktop, Cursor)
//   - Streamable HTTP: for containerised deployment accessible over the network
//
// This is an inbound adapter in the interface layer, peer to the REST and
// EPP adapters. It calls existing application services and contains no
// registry business logic.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// ---------------------------------------------------------------------------
// get_domain types
// ---------------------------------------------------------------------------

// GetDomainInput defines the input schema for the get_domain tool.
type GetDomainInput struct {
	Name string `json:"name" jsonschema:"Fully-qualified domain name to look up, e.g. example.best. Must include the TLD."`
}

// GetDomainOutput defines the structured output returned by the get_domain tool.
// Each field carries a jsonschema description that helps the host model interpret
// the registry semantics — treat these descriptions as part of the deliverable.
type GetDomainOutput struct {
	Name                string   `json:"name" jsonschema:"The queried domain name as stored in the registry (A-label for IDN domains)."`
	Statuses            []string `json:"statuses" jsonschema:"EPP status codes currently active on the domain per RFC 5731 section 2.3. Values include ok, inactive, clientHold, serverHold, clientTransferProhibited, serverTransferProhibited, clientUpdateProhibited, serverUpdateProhibited, clientDeleteProhibited, serverDeleteProhibited, clientRenewProhibited, serverRenewProhibited, pendingCreate, pendingRenew, pendingTransfer, pendingUpdate, pendingRestore, pendingDelete."`
	CreatedDate         string   `json:"createdDate" jsonschema:"Registry creation date in RFC 3339 format."`
	ExpiryDate          string   `json:"expiryDate" jsonschema:"Registry expiry date in RFC 3339 format. After this date the domain enters the renewal/redemption lifecycle."`
	RGPPhase            string   `json:"rgpPhase,omitempty" jsonschema:"Current Registry Grace Period phase if any. Possible values: addPeriod (grace after initial registration), renewPeriod (grace after manual renewal), autoRenewPeriod (grace after auto-renewal), transferPeriod (transfer lock period), redemptionPeriod (domain deleted but restorable), pendingDelete (past redemption and awaiting purge), pendingRestore (restore submitted and awaiting completion). Empty if no grace period is active."`
	RGPPhaseEndDate     string   `json:"rgpPhaseEndDate,omitempty" jsonschema:"Date when the current RGP phase ends in RFC 3339 format. Empty if no grace period is active or if not applicable to the current phase."`
	Nameservers         []string `json:"nameservers" jsonschema:"Fully-qualified hostnames of the nameservers delegated for this domain."`
	SponsoringRegistrar string   `json:"sponsoringRegistrar" jsonschema:"Client identifier (ClID) of the registrar currently sponsoring this domain. This is the EPP clIDType (3-16 ASCII characters)."`
}

// ---------------------------------------------------------------------------
// get_tld types
// ---------------------------------------------------------------------------

// GetTLDInput defines the input schema for the get_tld tool.
type GetTLDInput struct {
	Name string `json:"name" jsonschema:"TLD name to look up, e.g. best, radio, or xn--e1a4c (IDN). Do not include a leading dot."`
}

// GetTLDOutput defines the structured output returned by the get_tld tool.
type GetTLDOutput struct {
	Name               string        `json:"name" jsonschema:"The TLD name in ASCII (A-label)."`
	UnicodeName        string        `json:"unicodeName,omitempty" jsonschema:"The TLD name in Unicode (U-label) for internationalized TLDs. Empty for ASCII-only TLDs."`
	Type               string        `json:"type" jsonschema:"TLD classification. Values: generic (gTLD like .best or .radio), country-code (ccTLD like .nl or .uk), second-level (SLD like .co.uk)."`
	RegistryOperatorID string        `json:"registryOperatorId" jsonschema:"Registry Operator identifier (ClID) responsible for this TLD."`
	DNSEnabled         bool          `json:"dnsEnabled" jsonschema:"Whether DNS zone serving is enabled for this TLD."`
	CreatedDate        string        `json:"createdDate" jsonschema:"Date the TLD was created in the registry in RFC 3339 format."`
	UpdatedDate        string        `json:"updatedDate" jsonschema:"Date the TLD was last updated in RFC 3339 format."`
	CurrentPhases      []PhaseOutput `json:"currentPhases" jsonschema:"Currently active lifecycle phases. Includes the General Availability (GA) phase and any active Launch phases. Empty if no phases are active."`
}

// PhaseOutput represents a currently active TLD lifecycle phase.
type PhaseOutput struct {
	Name     string        `json:"name" jsonschema:"Phase identifier (ClIDType)."`
	Type     string        `json:"type" jsonschema:"Phase type: GA (General Availability) or Launch (pre-GA launch phase)."`
	Starts   string        `json:"starts" jsonschema:"Phase start date in RFC 3339 format."`
	Ends     string        `json:"ends,omitempty" jsonschema:"Phase end date in RFC 3339 format. Empty if the phase is open-ended."`
	Prices   []PriceOutput `json:"prices" jsonschema:"Per-currency pricing for domain operations in this phase. Amounts are in minor currency units (e.g. cents for USD)."`
	Policy   PolicyOutput  `json:"policy" jsonschema:"Phase policy defining registration rules and grace period durations."`
}

// PriceOutput represents per-currency pricing for a TLD phase.
type PriceOutput struct {
	Currency           string `json:"currency" jsonschema:"ISO 4217 currency code (e.g. USD, EUR)."`
	RegistrationAmount uint64 `json:"registrationAmount" jsonschema:"Registration price in minor currency units."`
	RenewalAmount      uint64 `json:"renewalAmount" jsonschema:"Renewal price in minor currency units."`
	TransferAmount     uint64 `json:"transferAmount" jsonschema:"Transfer price in minor currency units."`
	RestoreAmount      uint64 `json:"restoreAmount" jsonschema:"Restore-from-redemption price in minor currency units."`
}

// PolicyOutput represents the registration policy for a TLD phase.
type PolicyOutput struct {
	MinLabelLength     int    `json:"minLabelLength,omitempty" jsonschema:"Minimum domain label length in characters."`
	MaxLabelLength     int    `json:"maxLabelLength,omitempty" jsonschema:"Maximum domain label length in characters."`
	RegistrationGP     int    `json:"registrationGP,omitempty" jsonschema:"Registration grace period in days."`
	RenewalGP          int    `json:"renewalGP,omitempty" jsonschema:"Renewal grace period in days."`
	AutoRenewalGP      int    `json:"autoRenewalGP,omitempty" jsonschema:"Auto-renewal grace period in days."`
	TransferGP         int    `json:"transferGP,omitempty" jsonschema:"Transfer grace period in days."`
	RedemptionGP       int    `json:"redemptionGP,omitempty" jsonschema:"Redemption grace period in days."`
	PendingDeleteGP    int    `json:"pendingDeleteGP,omitempty" jsonschema:"Pending delete period in days."`
	TransferLockPeriod int    `json:"transferLockPeriod,omitempty" jsonschema:"Transfer lock period in days after a transfer completes."`
	MaxHorizon         int    `json:"maxHorizon,omitempty" jsonschema:"Maximum registration horizon in years."`
	AllowAutoRenew     *bool  `json:"allowAutoRenew,omitempty" jsonschema:"Whether auto-renewal is permitted in this phase."`
	BaseCurrency       string `json:"baseCurrency,omitempty" jsonschema:"Base currency for pricing in this phase (ISO 4217)."`
}

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

// Server is the MCP server adapter. It holds references to application services
// and exposes them as MCP tools. Additional services (RegistrarService, etc.)
// can be added here as new tools are implemented.
type Server struct {
	domainService interfaces.DomainService
	tldService    interfaces.TLDService
}

// NewServer creates a new MCP server adapter.
func NewServer(domainService interfaces.DomainService, tldService interfaces.TLDService) *Server {
	return &Server{
		domainService: domainService,
		tldService:    tldService,
	}
}

// Version is the MCP server version reported to clients during initialization.
const Version = "0.3.0"

// MCPServer creates the underlying MCP protocol server with all tools
// registered. The caller chooses the transport — stdio for local IDE use,
// or StreamableHTTPHandler for containerised network access.
func (s *Server) MCPServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "domain-os-mcp",
		Version: Version,
	}, nil)

	destructive := false
	openWorld := false
	readOnlyAnnotations := &mcp.ToolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: &destructive,
		IdempotentHint:  true,
		OpenWorldHint:   &openWorld,
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_domain",
		Description: "Look up the current registry state of a domain name, including EPP status codes, expiry, redemption/RGP state, nameservers, and sponsoring registrar.",
		Annotations: readOnlyAnnotations,
	}, s.GetDomain)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_tld",
		Description: "Look up the configuration and lifecycle state of a top-level domain (TLD), including type, registry operator, DNS status, and currently active phases with pricing and policy.",
		Annotations: readOnlyAnnotations,
	}, s.GetTLD)

	return server
}

// Run creates the MCP protocol server, registers all tools, and runs it over
// stdio. It blocks until the context is cancelled or the transport closes.
// For network (HTTP) transport, use MCPServer() directly with
// mcp.NewStreamableHTTPHandler.
func (s *Server) Run(ctx context.Context) error {
	return s.MCPServer().Run(ctx, &mcp.StdioTransport{})
}

// ---------------------------------------------------------------------------
// get_domain handler
// ---------------------------------------------------------------------------

// GetDomain handles the get_domain tool call. It validates the input,
// calls the DomainService, and maps the result to a structured output.
func (s *Server) GetDomain(ctx context.Context, req *mcp.CallToolRequest, in GetDomainInput) (*mcp.CallToolResult, GetDomainOutput, error) {
	// Normalize input
	name := strings.ToLower(strings.TrimSpace(in.Name))

	// Basic FQDN validation
	if name == "" || !strings.Contains(name, ".") {
		return nil, GetDomainOutput{}, fmt.Errorf("invalid domain name %q: must be a fully-qualified domain name containing at least one dot", in.Name)
	}

	// Call the existing application service
	dom, err := s.domainService.GetDomainByName(ctx, name, true)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			return nil, GetDomainOutput{}, fmt.Errorf("domain %q not found", name)
		}
		// Don't leak internal error details
		return nil, GetDomainOutput{}, fmt.Errorf("failed to look up domain %q", name)
	}

	// Derive the current RGP phase from status flags and RGP dates
	rgpPhase, rgpEndDate := deriveRGPPhase(dom)

	// Map the domain entity to the output struct
	output := GetDomainOutput{
		Name:                dom.Name.String(),
		Statuses:            dom.Status.StringSlice(),
		CreatedDate:         dom.CreatedAt.Format(time.RFC3339),
		ExpiryDate:          dom.ExpiryDate.Format(time.RFC3339),
		RGPPhase:            rgpPhase,
		RGPPhaseEndDate:     rgpEndDate,
		Nameservers:         dom.GetHostsAsStringSlice(),
		SponsoringRegistrar: dom.ClID.String(),
	}

	// Ensure nameservers is never null in JSON output
	if output.Nameservers == nil {
		output.Nameservers = []string{}
	}
	if output.Statuses == nil {
		output.Statuses = []string{}
	}

	return nil, output, nil
}

// deriveRGPPhase determines the current Registry Grace Period phase from the
// domain's status flags and RGP dates. This is a pure mapping — no business
// logic. It returns the phase name and end date (RFC 3339), or empty strings
// if no grace period is active.
func deriveRGPPhase(dom *entities.Domain) (phase, endDate string) {
	now := time.Now().UTC()

	// Pending restore — domain has been submitted for restore
	if dom.Status.PendingRestore {
		return "pendingRestore", ""
	}

	// Pending delete — check if still in redemption or past it
	if dom.Status.PendingDelete {
		if !dom.RGPStatus.RedemptionPeriodEnd.IsZero() && dom.RGPStatus.RedemptionPeriodEnd.After(now) {
			return "redemptionPeriod", dom.RGPStatus.RedemptionPeriodEnd.Format(time.RFC3339)
		}
		if !dom.RGPStatus.PurgeDate.IsZero() {
			return "pendingDelete", dom.RGPStatus.PurgeDate.Format(time.RFC3339)
		}
	}

	// Registration grace period
	if !dom.RGPStatus.AddPeriodEnd.IsZero() && dom.RGPStatus.AddPeriodEnd.After(now) {
		return "addPeriod", dom.RGPStatus.AddPeriodEnd.Format(time.RFC3339)
	}

	// Renewal grace period
	if !dom.RGPStatus.RenewPeriodEnd.IsZero() && dom.RGPStatus.RenewPeriodEnd.After(now) {
		return "renewPeriod", dom.RGPStatus.RenewPeriodEnd.Format(time.RFC3339)
	}

	// Auto-renewal grace period
	if !dom.RGPStatus.AutoRenewPeriodEnd.IsZero() && dom.RGPStatus.AutoRenewPeriodEnd.After(now) {
		return "autoRenewPeriod", dom.RGPStatus.AutoRenewPeriodEnd.Format(time.RFC3339)
	}

	// Transfer lock period
	if !dom.RGPStatus.TransferLockPeriodEnd.IsZero() && dom.RGPStatus.TransferLockPeriodEnd.After(now) {
		return "transferPeriod", dom.RGPStatus.TransferLockPeriodEnd.Format(time.RFC3339)
	}

	return "", ""
}

// ---------------------------------------------------------------------------
// get_tld handler
// ---------------------------------------------------------------------------

// GetTLD handles the get_tld tool call. It validates the input,
// calls the TLDService, and maps the result to a structured output.
func (s *Server) GetTLD(ctx context.Context, req *mcp.CallToolRequest, in GetTLDInput) (*mcp.CallToolResult, GetTLDOutput, error) {
	// Normalize input
	name := strings.ToLower(strings.TrimSpace(in.Name))
	name = strings.TrimPrefix(name, ".") // Remove leading dot if present

	if name == "" {
		return nil, GetTLDOutput{}, fmt.Errorf("invalid TLD name: must not be empty")
	}

	// Call the existing application service
	tld, err := s.tldService.GetTLDByName(ctx, name, true)
	if err != nil {
		if errors.Is(err, entities.ErrTLDNotFound) {
			return nil, GetTLDOutput{}, fmt.Errorf("TLD %q not found", name)
		}
		return nil, GetTLDOutput{}, fmt.Errorf("failed to look up TLD %q", name)
	}

	// Map current phases
	currentPhases := tld.GetCurrentPhases()
	phases := make([]PhaseOutput, 0, len(currentPhases))
	for i := range currentPhases {
		phases = append(phases, mapPhase(&currentPhases[i]))
	}

	output := GetTLDOutput{
		Name:               tld.Name.String(),
		UnicodeName:        tld.UName.String(),
		Type:               tld.Type.String(),
		RegistryOperatorID: tld.RyID.String(),
		DNSEnabled:         tld.EnableDNS,
		CreatedDate:        tld.CreatedAt.Format(time.RFC3339),
		UpdatedDate:        tld.UpdatedAt.Format(time.RFC3339),
		CurrentPhases:      phases,
	}

	return nil, output, nil
}

// mapPhase converts a domain Phase entity to a PhaseOutput struct.
func mapPhase(p *entities.Phase) PhaseOutput {
	out := PhaseOutput{
		Name:   p.Name.String(),
		Type:   string(p.Type),
		Starts: p.Starts.Format(time.RFC3339),
		Policy: PolicyOutput{
			MinLabelLength:     p.Policy.MinLabelLength,
			MaxLabelLength:     p.Policy.MaxLabelLength,
			RegistrationGP:     p.Policy.RegistrationGP,
			RenewalGP:          p.Policy.RenewalGP,
			AutoRenewalGP:      p.Policy.AutoRenewalGP,
			TransferGP:         p.Policy.TransferGP,
			RedemptionGP:       p.Policy.RedemptionGP,
			PendingDeleteGP:    p.Policy.PendingDeleteGP,
			TransferLockPeriod: p.Policy.TransferLockPeriod,
			MaxHorizon:         p.Policy.MaxHorizon,
			AllowAutoRenew:     p.Policy.AllowAutoRenew,
			BaseCurrency:       p.Policy.BaseCurrency,
		},
	}

	if p.Ends != nil {
		out.Ends = p.Ends.Format(time.RFC3339)
	}

	prices := make([]PriceOutput, 0, len(p.Prices))
	for _, pr := range p.Prices {
		prices = append(prices, PriceOutput{
			Currency:           pr.Currency,
			RegistrationAmount: pr.RegistrationAmount,
			RenewalAmount:      pr.RenewalAmount,
			TransferAmount:     pr.TransferAmount,
			RestoreAmount:      pr.RestoreAmount,
		})
	}
	out.Prices = prices

	return out
}
