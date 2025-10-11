package service

import (
	"testing"
)

// TestAddNavigationActions_TLDs tests navigation action generation for TLD-related queries
func TestAddNavigationActions_TLDs(t *testing.T) {
	s := &AgentService{}

	tests := []struct {
		name             string
		userMessage      string
		assistantMessage string
		expectedActions  int
		expectedLabel    string
		expectedPath     string
		expectedAutoNav  bool
	}{
		{
			name:             "show me all tlds - auto navigate",
			userMessage:      "show me all tlds",
			assistantMessage: "Here are the TLDs in the system",
			expectedActions:  1,
			expectedLabel:    "View All TLDs",
			expectedPath:     "/tlds",
			expectedAutoNav:  true,
		},
		{
			name:             "list tlds - auto navigate",
			userMessage:      "open the list tlds page",
			assistantMessage: "I'll help you view the TLDs",
			expectedActions:  1,
			expectedLabel:    "View All TLDs",
			expectedPath:     "/tlds",
			expectedAutoNav:  true,
		},
		{
			name:             "show tlds without trigger - no auto navigate",
			userMessage:      "can you show tlds?",
			assistantMessage: "TLDs are top-level domains",
			expectedActions:  1,
			expectedLabel:    "View All TLDs",
			expectedPath:     "/tlds",
			expectedAutoNav:  false,
		},
		{
			name:             "general tld mention - no action",
			userMessage:      "what is a tld?",
			assistantMessage: "A TLD is a top-level domain",
			expectedActions:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &ChatResponse{}
			s.addNavigationActions(response, tt.userMessage, tt.assistantMessage)

			if len(response.Actions) != tt.expectedActions {
				t.Errorf("Expected %d actions, got %d", tt.expectedActions, len(response.Actions))
				return
			}

			if tt.expectedActions > 0 {
				action := response.Actions[0]
				if action.Label != tt.expectedLabel {
					t.Errorf("Expected label '%s', got '%s'", tt.expectedLabel, action.Label)
				}
				if action.Path != tt.expectedPath {
					t.Errorf("Expected path '%s', got '%s'", tt.expectedPath, action.Path)
				}
				if action.AutoNavigate != tt.expectedAutoNav {
					t.Errorf("Expected autoNavigate %v, got %v", tt.expectedAutoNav, action.AutoNavigate)
				}
				if action.Type != "navigate" {
					t.Errorf("Expected type 'navigate', got '%s'", action.Type)
				}
				if action.Variant != "default" {
					t.Errorf("Expected variant 'default', got '%s'", action.Variant)
				}
			}
		})
	}
}

// TestAddNavigationActions_RegistryOperators tests navigation for registry operator queries
func TestAddNavigationActions_RegistryOperators(t *testing.T) {
	s := &AgentService{}

	tests := []struct {
		name             string
		userMessage      string
		assistantMessage string
		expectedActions  int
		expectedLabel    string
		expectedPath     string
		expectedAutoNav  bool
	}{
		{
			name:             "show all registry operators",
			userMessage:      "show me all registry operators",
			assistantMessage: "Here are the registry operators",
			expectedActions:  1,
			expectedLabel:    "View All Registry Operators",
			expectedPath:     "/registry-operators",
			expectedAutoNav:  true,
		},
		{
			name:             "list operators with go to",
			userMessage:      "go to the list of operators",
			assistantMessage: "Navigating to operators",
			expectedActions:  1,
			expectedLabel:    "View All Registry Operators",
			expectedPath:     "/registry-operators",
			expectedAutoNav:  true,
		},
		{
			name:             "operators question with show",
			userMessage:      "can you show me operators?",
			assistantMessage: "Registry operators manage TLDs",
			expectedActions:  1,
			expectedLabel:    "View All Registry Operators",
			expectedPath:     "/registry-operators",
			expectedAutoNav:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &ChatResponse{}
			s.addNavigationActions(response, tt.userMessage, tt.assistantMessage)

			if len(response.Actions) != tt.expectedActions {
				t.Errorf("Expected %d actions, got %d", tt.expectedActions, len(response.Actions))
				return
			}

			if tt.expectedActions > 0 {
				action := response.Actions[0]
				if action.Label != tt.expectedLabel {
					t.Errorf("Expected label '%s', got '%s'", tt.expectedLabel, action.Label)
				}
				if action.Path != tt.expectedPath {
					t.Errorf("Expected path '%s', got '%s'", tt.expectedPath, action.Path)
				}
				if action.AutoNavigate != tt.expectedAutoNav {
					t.Errorf("Expected autoNavigate %v, got %v", tt.expectedAutoNav, action.AutoNavigate)
				}
			}
		})
	}
}

// TestAddNavigationActions_Domains tests navigation for domain queries
func TestAddNavigationActions_Domains(t *testing.T) {
	s := &AgentService{}

	tests := []struct {
		name             string
		userMessage      string
		assistantMessage string
		expectedActions  int
		expectedLabel    string
		expectedPath     string
		expectedAutoNav  bool
	}{
		{
			name:             "show all domains",
			userMessage:      "show me all domains",
			assistantMessage: "Here are the domains",
			expectedActions:  1,
			expectedLabel:    "View All Domains",
			expectedPath:     "/domains",
			expectedAutoNav:  true,
		},
		{
			name:             "list domains with navigate",
			userMessage:      "navigate to list domains",
			assistantMessage: "Opening domains page",
			expectedActions:  1,
			expectedLabel:    "View All Domains",
			expectedPath:     "/domains",
			expectedAutoNav:  true,
		},
		{
			name:             "domain question without navigation trigger",
			userMessage:      "how do I register a domain?",
			assistantMessage: "To register a domain...",
			expectedActions:  0,
		},
		{
			name:             "domain tld mention - should not trigger domain navigation",
			userMessage:      "show me all tlds",
			assistantMessage: "Here are the TLDs",
			expectedActions:  1,
			expectedLabel:    "View All TLDs", // Should be TLDs, not domains
			expectedPath:     "/tlds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &ChatResponse{}
			s.addNavigationActions(response, tt.userMessage, tt.assistantMessage)

			if len(response.Actions) != tt.expectedActions {
				t.Errorf("Expected %d actions, got %d", tt.expectedActions, len(response.Actions))
				return
			}

			if tt.expectedActions > 0 {
				action := response.Actions[0]
				if action.Label != tt.expectedLabel {
					t.Errorf("Expected label '%s', got '%s'", tt.expectedLabel, action.Label)
				}
				if action.Path != tt.expectedPath {
					t.Errorf("Expected path '%s', got '%s'", tt.expectedPath, action.Path)
				}
				if tt.expectedAutoNav && action.AutoNavigate != tt.expectedAutoNav {
					t.Errorf("Expected autoNavigate %v, got %v", tt.expectedAutoNav, action.AutoNavigate)
				}
			}
		})
	}
}

// TestAddNavigationActions_Dashboard tests dashboard navigation
func TestAddNavigationActions_Dashboard(t *testing.T) {
	s := &AgentService{}

	tests := []struct {
		name             string
		userMessage      string
		assistantMessage string
		expectedActions  int
		expectedLabel    string
		expectedPath     string
		expectedAutoNav  bool
	}{
		{
			name:             "go to dashboard",
			userMessage:      "take me to the dashboard",
			assistantMessage: "Opening dashboard",
			expectedActions:  1,
			expectedLabel:    "Go to Dashboard",
			expectedPath:     "/",
			expectedAutoNav:  true,
		},
		{
			name:             "show home page",
			userMessage:      "show me the home page",
			assistantMessage: "Here's the home page",
			expectedActions:  1,
			expectedLabel:    "Go to Dashboard",
			expectedPath:     "/",
			expectedAutoNav:  true,
		},
		{
			name:             "overview request",
			userMessage:      "open the overview",
			assistantMessage: "Opening overview",
			expectedActions:  1,
			expectedLabel:    "Go to Dashboard",
			expectedPath:     "/",
			expectedAutoNav:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &ChatResponse{}
			s.addNavigationActions(response, tt.userMessage, tt.assistantMessage)

			if len(response.Actions) != tt.expectedActions {
				t.Errorf("Expected %d actions, got %d", tt.expectedActions, len(response.Actions))
				return
			}

			if tt.expectedActions > 0 {
				action := response.Actions[0]
				if action.Label != tt.expectedLabel {
					t.Errorf("Expected label '%s', got '%s'", tt.expectedLabel, action.Label)
				}
				if action.Path != tt.expectedPath {
					t.Errorf("Expected path '%s', got '%s'", tt.expectedPath, action.Path)
				}
				if action.AutoNavigate != tt.expectedAutoNav {
					t.Errorf("Expected autoNavigate %v, got %v", tt.expectedAutoNav, action.AutoNavigate)
				}
			}
		})
	}
}

// TestAddNavigationActions_AutoNavigateTriggers tests all auto-navigation trigger phrases
func TestAddNavigationActions_AutoNavigateTriggers(t *testing.T) {
	s := &AgentService{}

	triggers := []string{
		"show me all tlds",
		"open the tlds page",
		"go to tlds",
		"navigate to the tlds",
		"take me to the tlds page",
	}

	for _, trigger := range triggers {
		t.Run(trigger, func(t *testing.T) {
			response := &ChatResponse{}
			s.addNavigationActions(response, trigger, "Here are the TLDs")

			if len(response.Actions) != 1 {
				t.Errorf("Expected 1 action for trigger '%s', got %d", trigger, len(response.Actions))
				return
			}

			if !response.Actions[0].AutoNavigate {
				t.Errorf("Expected autoNavigate=true for trigger '%s'", trigger)
			}
		})
	}
}

// TestAddNavigationActions_CaseInsensitive tests that matching is case-insensitive
func TestAddNavigationActions_CaseInsensitive(t *testing.T) {
	s := &AgentService{}

	tests := []struct {
		name            string
		userMessage     string
		expectedActions int
	}{
		{
			name:            "lowercase",
			userMessage:     "show me all tlds",
			expectedActions: 1,
		},
		{
			name:            "uppercase",
			userMessage:     "SHOW ME ALL TLDS",
			expectedActions: 1,
		},
		{
			name:            "mixed case",
			userMessage:     "ShOw Me AlL TLDs",
			expectedActions: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &ChatResponse{}
			s.addNavigationActions(response, tt.userMessage, "Here are the TLDs")

			if len(response.Actions) != tt.expectedActions {
				t.Errorf("Expected %d actions, got %d", tt.expectedActions, len(response.Actions))
			}
		})
	}
}

// TestAddNavigationActions_NoActions tests scenarios where no actions should be added
func TestAddNavigationActions_NoActions(t *testing.T) {
	s := &AgentService{}

	tests := []struct {
		name             string
		userMessage      string
		assistantMessage string
	}{
		{
			name:             "generic question",
			userMessage:      "what can you help me with?",
			assistantMessage: "I can help with many things",
		},
		{
			name:             "function execution",
			userMessage:      "create a new tld called .example",
			assistantMessage: "I'll create that TLD for you",
		},
		{
			name:             "empty messages",
			userMessage:      "",
			assistantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &ChatResponse{}
			s.addNavigationActions(response, tt.userMessage, tt.assistantMessage)

			if len(response.Actions) != 0 {
				t.Errorf("Expected 0 actions, got %d", len(response.Actions))
			}
		})
	}
}

// TestNavigationActionStruct tests the NavigationAction struct fields
func TestNavigationActionStruct(t *testing.T) {
	action := NavigationAction{
		Type:         "navigate",
		Label:        "Test Label",
		Path:         "/test",
		Variant:      "default",
		AutoNavigate: true,
	}

	if action.Type != "navigate" {
		t.Errorf("Expected Type 'navigate', got '%s'", action.Type)
	}
	if action.Label != "Test Label" {
		t.Errorf("Expected Label 'Test Label', got '%s'", action.Label)
	}
	if action.Path != "/test" {
		t.Errorf("Expected Path '/test', got '%s'", action.Path)
	}
	if action.Variant != "default" {
		t.Errorf("Expected Variant 'default', got '%s'", action.Variant)
	}
	if !action.AutoNavigate {
		t.Errorf("Expected AutoNavigate true, got false")
	}
}

// TestChatResponse_WithActions tests ChatResponse with navigation actions
func TestChatResponse_WithActions(t *testing.T) {
	response := ChatResponse{
		Message:        "Here are the TLDs",
		ConversationID: "test-123",
		Actions: []NavigationAction{
			{
				Type:         "navigate",
				Label:        "View TLDs",
				Path:         "/tlds",
				Variant:      "default",
				AutoNavigate: true,
			},
		},
	}

	if response.Message != "Here are the TLDs" {
		t.Errorf("Expected message 'Here are the TLDs', got '%s'", response.Message)
	}
	if response.ConversationID != "test-123" {
		t.Errorf("Expected conversation ID 'test-123', got '%s'", response.ConversationID)
	}
	if len(response.Actions) != 1 {
		t.Errorf("Expected 1 action, got %d", len(response.Actions))
	}
	if response.Actions[0].Label != "View TLDs" {
		t.Errorf("Expected action label 'View TLDs', got '%s'", response.Actions[0].Label)
	}
}

// TestChatResponse_WithoutActions tests ChatResponse without navigation actions
func TestChatResponse_WithoutActions(t *testing.T) {
	response := ChatResponse{
		Message:        "Hello!",
		ConversationID: "test-456",
	}

	if len(response.Actions) != 0 {
		t.Errorf("Expected 0 actions, got %d", len(response.Actions))
	}
}
