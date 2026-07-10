package askg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/interface/mcp"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// InProcessToolExecutor executes tool calls by calling application services
// directly — no MCP transport round-trip. It reuses the same input/output
// structs and mapping logic as the MCP server so schemas never drift.
type InProcessToolExecutor struct {
	domainService    interfaces.DomainService
	tldService       interfaces.TLDService
	knowledgeService interfaces.KnowledgeService // nil if knowledge base not loaded
	logger           *slog.Logger
}

// NewInProcessToolExecutor creates a tool executor wired to the given
// application services. The knowledgeService may be nil — if so, the
// answer_system_question tool is simply not advertised.
// The logger is used for structured observability.
func NewInProcessToolExecutor(
	domainService interfaces.DomainService,
	tldService interfaces.TLDService,
	knowledgeService interfaces.KnowledgeService,
	logger *slog.Logger,
) *InProcessToolExecutor {
	return &InProcessToolExecutor{
		domainService:    domainService,
		tldService:       tldService,
		knowledgeService: knowledgeService,
		logger:           logger,
	}
}

// Tools returns the tool definitions derived from the MCP tool types.
// The JSON Schema is built from the same struct tags used by the MCP server.
// The answer_system_question tool is only advertised when a KnowledgeService
// is configured.
func (e *InProcessToolExecutor) Tools() []ToolDef {
	tools := []ToolDef{
		{
			Name:        "get_domain",
			Description: "Look up the current registry state of a domain name, including EPP status codes, expiry, redemption/RGP state, nameservers, and sponsoring registrar.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Fully-qualified domain name to look up, e.g. example.best. Must include the TLD.",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "get_tld",
			Description: "Look up the configuration and lifecycle state of a top-level domain (TLD), including type, registry operator, DNS status, and currently active phases with pricing and policy.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "TLD name to look up, e.g. best, radio, or xn--e1a4c (IDN). Do not include a leading dot.",
					},
				},
				"required": []string{"name"},
			},
		},
	}

	if e.knowledgeService != nil {
		tools = append(tools, ToolDef{
			Name:        "answer_system_question",
			Description: "Search the internal knowledge base for information about system architecture, workflows, deployment, domain lifecycle, and registry operations. Use this when the question is about HOW the system works, not about specific domain or TLD data.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "The question to search the knowledge base for",
					},
				},
				"required": []string{"question"},
			},
		})
	}

	return tools
}

// Execute runs a single tool call against the in-process application services.
func (e *InProcessToolExecutor) Execute(ctx context.Context, call ToolCall, scope CallerScope) ToolResult {
	start := time.Now()

	e.logger.InfoContext(ctx, "executing tool",
		slog.String("tool", call.Name),
		slog.String("call_id", call.ID),
		slog.String("caller", scope.UserID),
		slog.Any("input", call.Input),
	)

	var result any
	var err error

	switch call.Name {
	case "get_domain":
		result, err = e.executeDomain(ctx, call.Input)
	case "get_tld":
		result, err = e.executeTLD(ctx, call.Input)
	case "answer_system_question":
		result, err = e.executeKnowledgeSearch(ctx, call.Input)
	default:
		err = fmt.Errorf("unknown tool: %s", call.Name)
	}

	latency := time.Since(start)

	if err != nil {
		e.logger.WarnContext(ctx, "tool execution failed",
			slog.String("tool", call.Name),
			slog.String("call_id", call.ID),
			slog.Duration("latency", latency),
			slog.String("error", err.Error()),
		)
		return ToolResult{
			CallID:  call.ID,
			Result:  map[string]string{"error": err.Error()},
			IsError: true,
		}
	}

	e.logger.InfoContext(ctx, "tool execution succeeded",
		slog.String("tool", call.Name),
		slog.String("call_id", call.ID),
		slog.Duration("latency", latency),
	)

	return ToolResult{
		CallID:  call.ID,
		Result:  result,
		IsError: false,
	}
}

// executeDomain mirrors the MCP server's GetDomain handler, calling the
// same DomainService method and using the same output mapping.
func (e *InProcessToolExecutor) executeDomain(ctx context.Context, rawInput any) (*mcp.GetDomainOutput, error) {
	input, err := extractStringField(rawInput, "name")
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	name := strings.ToLower(strings.TrimSpace(input))
	if name == "" || !strings.Contains(name, ".") {
		return nil, fmt.Errorf("invalid domain name %q: must be a fully-qualified domain name containing at least one dot", input)
	}

	dom, err := e.domainService.GetDomainByName(ctx, name, true)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			return nil, fmt.Errorf("domain %q not found", name)
		}
		return nil, fmt.Errorf("failed to look up domain %q", name)
	}

	rgpPhase, rgpEndDate := deriveDomainRGPPhase(dom)

	output := &mcp.GetDomainOutput{
		Name:                dom.Name.String(),
		Statuses:            dom.Status.StringSlice(),
		CreatedDate:         dom.CreatedAt.Format(time.RFC3339),
		ExpiryDate:          dom.ExpiryDate.Format(time.RFC3339),
		RGPPhase:            rgpPhase,
		RGPPhaseEndDate:     rgpEndDate,
		Nameservers:         dom.GetHostsAsStringSlice(),
		SponsoringRegistrar: dom.ClID.String(),
	}

	if output.Nameservers == nil {
		output.Nameservers = []string{}
	}
	if output.Statuses == nil {
		output.Statuses = []string{}
	}

	return output, nil
}

// executeTLD mirrors the MCP server's GetTLD handler.
func (e *InProcessToolExecutor) executeTLD(ctx context.Context, rawInput any) (*mcp.GetTLDOutput, error) {
	input, err := extractStringField(rawInput, "name")
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	name := strings.ToLower(strings.TrimSpace(input))
	name = strings.TrimPrefix(name, ".")
	if name == "" {
		return nil, fmt.Errorf("invalid TLD name: must not be empty")
	}

	tld, err := e.tldService.GetTLDByName(ctx, name, true)
	if err != nil {
		if errors.Is(err, entities.ErrTLDNotFound) {
			return nil, fmt.Errorf("TLD %q not found", name)
		}
		return nil, fmt.Errorf("failed to look up TLD %q", name)
	}

	currentPhases := tld.GetCurrentPhases()
	phases := make([]mcp.PhaseOutput, 0, len(currentPhases))
	for i := range currentPhases {
		phases = append(phases, mapPhaseToOutput(&currentPhases[i]))
	}

	output := &mcp.GetTLDOutput{
		Name:               tld.Name.String(),
		UnicodeName:        tld.UName.String(),
		Type:               tld.Type.String(),
		RegistryOperatorID: tld.RyID.String(),
		DNSEnabled:         tld.EnableDNS,
		CreatedDate:        tld.CreatedAt.Format(time.RFC3339),
		UpdatedDate:        tld.UpdatedAt.Format(time.RFC3339),
		CurrentPhases:      phases,
	}

	return output, nil
}

// knowledgeSearchOutput is the structured output for the answer_system_question tool.
type knowledgeSearchOutput struct {
	LowConfidence bool                   `json:"low_confidence"`
	Message       string                 `json:"message,omitempty"`
	Results       []knowledgeSearchEntry  `json:"results,omitempty"`
}

// knowledgeSearchEntry is a single chunk from the knowledge search results.
type knowledgeSearchEntry struct {
	Source  string  `json:"source"`  // relative file path
	Section string  `json:"section"` // heading trail
	Content string  `json:"content"` // markdown text
	Score   float64 `json:"score"`   // BM25 relevance score
}

// executeKnowledgeSearch searches the knowledge base for the given question
// and returns the top matching chunks. If no results are found or all scores
// are below the confidence threshold, it returns a low_confidence flag.
func (e *InProcessToolExecutor) executeKnowledgeSearch(_ context.Context, rawInput any) (*knowledgeSearchOutput, error) {
	if e.knowledgeService == nil {
		return nil, fmt.Errorf("knowledge base not available: KnowledgeService was not initialized — check that docs/index.yaml exists and is valid")
	}

	question, err := extractStringField(rawInput, "question")
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	question = strings.TrimSpace(question)
	if question == "" {
		return nil, fmt.Errorf("invalid input: question must not be empty")
	}

	results, err := e.knowledgeService.Search(question, 5)
	if err != nil {
		return nil, fmt.Errorf("knowledge search failed for question %q: %w", question, err)
	}

	// Check for low-confidence: no results or all scores below threshold.
	const confidenceThreshold = 0.5

	if len(results) == 0 || results[0].Score < confidenceThreshold {
		return &knowledgeSearchOutput{
			LowConfidence: true,
			Message:       "No sufficiently relevant documentation was found for this question. You should escalate to a human rather than attempting to answer from these results.",
		}, nil
	}

	entries := make([]knowledgeSearchEntry, 0, len(results))
	for _, r := range results {
		if r.Score < confidenceThreshold {
			break // results are sorted by score, so stop at first low-confidence
		}
		entries = append(entries, knowledgeSearchEntry{
			Source:  r.DocPath,
			Section: r.Section,
			Content: r.Content,
			Score:   r.Score,
		})
	}

	return &knowledgeSearchOutput{
		LowConfidence: false,
		Results:       entries,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractStringField extracts a string field from a JSON-decoded input.
// The input may be a map[string]any or a JSON-encoded string.
func extractStringField(rawInput any, field string) (string, error) {
	switch v := rawInput.(type) {
	case map[string]any:
		val, ok := v[field]
		if !ok {
			return "", fmt.Errorf("missing required field %q", field)
		}
		s, ok := val.(string)
		if !ok {
			return "", fmt.Errorf("field %q must be a string", field)
		}
		return s, nil
	case string:
		// Try to parse as JSON
		var m map[string]any
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			return "", fmt.Errorf("cannot parse input as JSON: %w", err)
		}
		return extractStringField(m, field)
	default:
		return "", fmt.Errorf("unexpected input type %T", rawInput)
	}
}

// deriveDomainRGPPhase mirrors the MCP server's deriveRGPPhase function.
// It determines the current Registry Grace Period phase from the domain's
// status flags and RGP dates.
func deriveDomainRGPPhase(dom *entities.Domain) (phase, endDate string) {
	now := time.Now().UTC()

	if dom.Status.PendingRestore {
		return "pendingRestore", ""
	}

	if dom.Status.PendingDelete {
		if !dom.RGPStatus.RedemptionPeriodEnd.IsZero() && dom.RGPStatus.RedemptionPeriodEnd.After(now) {
			return "redemptionPeriod", dom.RGPStatus.RedemptionPeriodEnd.Format(time.RFC3339)
		}
		if !dom.RGPStatus.PurgeDate.IsZero() {
			return "pendingDelete", dom.RGPStatus.PurgeDate.Format(time.RFC3339)
		}
	}

	if !dom.RGPStatus.AddPeriodEnd.IsZero() && dom.RGPStatus.AddPeriodEnd.After(now) {
		return "addPeriod", dom.RGPStatus.AddPeriodEnd.Format(time.RFC3339)
	}

	if !dom.RGPStatus.RenewPeriodEnd.IsZero() && dom.RGPStatus.RenewPeriodEnd.After(now) {
		return "renewPeriod", dom.RGPStatus.RenewPeriodEnd.Format(time.RFC3339)
	}

	if !dom.RGPStatus.AutoRenewPeriodEnd.IsZero() && dom.RGPStatus.AutoRenewPeriodEnd.After(now) {
		return "autoRenewPeriod", dom.RGPStatus.AutoRenewPeriodEnd.Format(time.RFC3339)
	}

	if !dom.RGPStatus.TransferLockPeriodEnd.IsZero() && dom.RGPStatus.TransferLockPeriodEnd.After(now) {
		return "transferPeriod", dom.RGPStatus.TransferLockPeriodEnd.Format(time.RFC3339)
	}

	return "", ""
}

// mapPhaseToOutput converts a domain Phase entity to a PhaseOutput struct.
// This mirrors the MCP server's mapPhase function.
func mapPhaseToOutput(p *entities.Phase) mcp.PhaseOutput {
	out := mcp.PhaseOutput{
		Name:   p.Name.String(),
		Type:   string(p.Type),
		Starts: p.Starts.Format(time.RFC3339),
		Policy: mcp.PolicyOutput{
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

	prices := make([]mcp.PriceOutput, 0, len(p.Prices))
	for _, pr := range p.Prices {
		prices = append(prices, mcp.PriceOutput{
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
