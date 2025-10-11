package functions

import (
	"encoding/json"
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/agent/client"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// Functions provides agent function implementations
type Functions struct {
	adminClient *client.AdminAPIClient
}

// NewFunctions creates a new Functions instance
func NewFunctions(adminClient *client.AdminAPIClient) *Functions {
	return &Functions{
		adminClient: adminClient,
	}
}

// GetFunctionDefinitions returns all available agent functions
func (f *Functions) GetFunctionDefinitions() []openai.Tool {
	return []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "create_registry_operator",
				Description: "Create a new registry operator organization. Use this when the user wants to set up a new RO. The RyID is a unique identifier/handle for the registry operator.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"ryID": {
							Type:        jsonschema.String,
							Description: "Registry operator ID/handle (e.g., 'AlpacaNames'). This should be a unique identifier without spaces.",
						},
						"name": {
							Type:        jsonschema.String,
							Description: "Registry operator name (e.g., 'Acme Registry')",
						},
						"email": {
							Type:        jsonschema.String,
							Description: "Contact email address",
						},
						"url": {
							Type:        jsonschema.String,
							Description: "Website URL (optional)",
						},
					},
					Required: []string{"ryID", "name", "email"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "list_registry_operators",
				Description: "List all registry operators in the system. Use this to show available ROs or when user asks about existing registry operators.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"limit": {
							Type:        jsonschema.Number,
							Description: "Maximum number of results to return (default: 50)",
						},
					},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "create_tld",
				Description: "Create a new top-level domain under a registry operator. Use this when the user wants to set up a new TLD like .shop or .brand. The TLD type (generic/geographic/sponsored) is automatically determined by the backend.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"tld": {
							Type:        jsonschema.String,
							Description: "TLD name without dot (e.g., 'shop', 'brand', 'alpaca')",
						},
						"registry_operator_id": {
							Type:        jsonschema.String,
							Description: "Registry operator RyID (e.g., 'AlpacaNames')",
						},
					},
					Required: []string{"tld", "registry_operator_id"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "list_tlds",
				Description: "List all TLDs in the system. Use this to show available TLDs or when user asks about existing top-level domains.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"limit": {
							Type:        jsonschema.Number,
							Description: "Maximum number of results to return (default: 50)",
						},
					},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "create_phase",
				Description: "Create a new phase for a TLD (e.g., Sunrise, Landrush, General Availability). Use this when setting up TLD launch phases.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"name": {
							Type:        jsonschema.String,
							Description: "Phase name (e.g., 'Sunrise', 'Landrush', 'General Availability'). This is a descriptive name for the phase.",
						},
						"tld_name": {
							Type:        jsonschema.String,
							Description: "TLD name without dot (e.g., 'alpaca', 'my.alpaca')",
						},
						"type": {
							Type:        jsonschema.String,
							Description: "Phase type: 'Launch' for launch phases (Sunrise, Landrush, EAP, etc.) or 'GA' for General Availability",
							Enum:        []string{"Launch", "GA"},
						},
						"starts": {
							Type:        jsonschema.String,
							Description: "Start date in ISO 8601 format (e.g., '2025-01-15T00:00:00Z')",
						},
						"ends": {
							Type:        jsonschema.String,
							Description: "End date in ISO 8601 format (optional for open-ended phases)",
						},
					},
					Required: []string{"name", "tld_name", "type", "starts"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "end_phase",
				Description: "Set or update the end date for a phase. Use this to close a phase or set when it should end.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"tld_name": {
							Type:        jsonschema.String,
							Description: "TLD name without dot (e.g., 'alpaca', 'my.alpaca')",
						},
						"phase_name": {
							Type:        jsonschema.String,
							Description: "Phase name (e.g., 'Sunrise', 'General Availability')",
						},
						"ends": {
							Type:        jsonschema.String,
							Description: "End date in ISO 8601 format (e.g., '2026-03-01T00:00:00Z'). Must be after the phase start date and in the future.",
						},
					},
					Required: []string{"tld_name", "phase_name", "ends"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "list_phases",
				Description: "List all phases for a specific TLD. Use this to show phase information for a TLD.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"tld_name": {
							Type:        jsonschema.String,
							Description: "TLD name without dot (e.g., 'alpaca', 'my.alpaca')",
						},
						"limit": {
							Type:        jsonschema.Number,
							Description: "Maximum number of results to return (default: 50)",
						},
					},
					Required: []string{"tld_name"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "get_tld_info",
				Description: "Get detailed information about a specific TLD. Use this when user asks about a TLD's details, status, or configuration.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"tld": {
							Type:        jsonschema.String,
							Description: "TLD name without dot (e.g., 'shop', 'com')",
						},
					},
					Required: []string{"tld"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "search_domains",
				Description: "Search for domains by name pattern. Use this when user wants to find specific domains or check availability.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"query": {
							Type:        jsonschema.String,
							Description: "Search query (domain name or pattern)",
						},
						"limit": {
							Type:        jsonschema.Number,
							Description: "Maximum number of results to return (default: 20)",
						},
					},
					Required: []string{"query"},
				},
			},
		},
	}
}

// ExecuteFunction executes a function by name with the given arguments
func (f *Functions) ExecuteFunction(name string, arguments string) (string, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	switch name {
	case "create_registry_operator":
		return f.createRegistryOperator(args)
	case "list_registry_operators":
		return f.listRegistryOperators(args)
	case "create_tld":
		return f.createTLD(args)
	case "list_tlds":
		return f.listTLDs(args)
	case "create_phase":
		return f.createPhase(args)
	case "end_phase":
		return f.endPhase(args)
	case "list_phases":
		return f.listPhases(args)
	case "get_tld_info":
		return f.getTLDInfo(args)
	case "search_domains":
		return f.searchDomains(args)
	default:
		return "", fmt.Errorf("unknown function: %s", name)
	}
}

// Implementation of each function

func (f *Functions) createRegistryOperator(args map[string]interface{}) (string, error) {
	body := map[string]interface{}{
		"ryID":  args["ryID"],
		"name":  args["name"],
		"email": args["email"],
	}
	if url, ok := args["url"].(string); ok && url != "" {
		body["url"] = url
	}

	resp, err := f.adminClient.Post("/registry-operators", body)
	if err != nil {
		return "", fmt.Errorf("failed to create registry operator: %w", err)
	}

	return fmt.Sprintf("Successfully created registry operator '%s' (RyID: %s). Response: %s", args["name"], args["ryID"], string(resp)), nil
}

func (f *Functions) listRegistryOperators(args map[string]interface{}) (string, error) {
	limit := 50
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	path := fmt.Sprintf("/registry-operators?limit=%d", limit)
	resp, err := f.adminClient.Get(path)
	if err != nil {
		return "", fmt.Errorf("failed to list registry operators: %w", err)
	}

	return string(resp), nil
}

func (f *Functions) createTLD(args map[string]interface{}) (string, error) {
	body := map[string]interface{}{
		"Name": args["tld"],
		"RyID": args["registry_operator_id"],
	}

	resp, err := f.adminClient.Post("/tlds", body)
	if err != nil {
		return "", fmt.Errorf("failed to create TLD: %w", err)
	}

	return fmt.Sprintf("Successfully created TLD .%s under registry operator %s. Response: %s", args["tld"], args["registry_operator_id"], string(resp)), nil
}

func (f *Functions) listTLDs(args map[string]interface{}) (string, error) {
	limit := 50
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	path := fmt.Sprintf("/tlds?limit=%d", limit)
	resp, err := f.adminClient.Get(path)
	if err != nil {
		return "", fmt.Errorf("failed to list TLDs: %w", err)
	}

	return string(resp), nil
}

func (f *Functions) createPhase(args map[string]interface{}) (string, error) {
	tldName := args["tld_name"].(string)

	body := map[string]interface{}{
		"name":   args["name"],
		"type":   args["type"],
		"starts": args["starts"],
	}
	if ends, ok := args["ends"].(string); ok && ends != "" {
		body["ends"] = ends
	}

	path := fmt.Sprintf("/tlds/%s/phases", tldName)
	resp, err := f.adminClient.Post(path, body)
	if err != nil {
		return "", fmt.Errorf("failed to create phase: %w", err)
	}

	return fmt.Sprintf("Successfully created phase '%s' for TLD '%s'. Response: %s", args["name"], tldName, string(resp)), nil
}

func (f *Functions) endPhase(args map[string]interface{}) (string, error) {
	tldName := args["tld_name"].(string)
	phaseName := args["phase_name"].(string)

	body := map[string]interface{}{
		"ends": args["ends"],
	}

	path := fmt.Sprintf("/tlds/%s/phases/%s/end", tldName, phaseName)
	resp, err := f.adminClient.Put(path, body)
	if err != nil {
		return "", fmt.Errorf("failed to set end date for phase: %w", err)
	}

	return fmt.Sprintf("Successfully set end date for phase '%s' on TLD '%s'. Response: %s", phaseName, tldName, string(resp)), nil
}

func (f *Functions) listPhases(args map[string]interface{}) (string, error) {
	tldName := args["tld_name"].(string)

	limit := 50
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	path := fmt.Sprintf("/tlds/%s/phases?limit=%d", tldName, limit)

	resp, err := f.adminClient.Get(path)
	if err != nil {
		return "", fmt.Errorf("failed to list phases for TLD '%s': %w", tldName, err)
	}

	return string(resp), nil
}

func (f *Functions) getTLDInfo(args map[string]interface{}) (string, error) {
	tld := args["tld"].(string)
	path := fmt.Sprintf("/tlds/%s", tld)

	resp, err := f.adminClient.Get(path)
	if err != nil {
		return "", fmt.Errorf("failed to get TLD info: %w", err)
	}

	return string(resp), nil
}

func (f *Functions) searchDomains(args map[string]interface{}) (string, error) {
	query := args["query"].(string)
	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	path := fmt.Sprintf("/domains?q=%s&limit=%d", query, limit)
	resp, err := f.adminClient.Get(path)
	if err != nil {
		return "", fmt.Errorf("failed to search domains: %w", err)
	}

	return string(resp), nil
}
