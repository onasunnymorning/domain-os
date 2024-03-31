package entities

import (
	"testing"
	"time"
)

func TestNewTLD(t *testing.T) {
	tests := []struct {
		name     string
		expected *TLD
		err      error
	}{
		{
			name:     "example.com",
			expected: &TLD{Name: "example.com"},
			err:      nil,
		},
		{
			name:     "-invalid",
			expected: nil,
			err:      ErrInvalidLabelDash,
		},
	}

	for _, test := range tests {
		result, err := NewTLD(test.name)
		if err != test.err {
			t.Errorf("Expected error to be %v, but got %v for input %s", test.err, err, test.name)
		}
		if result != nil && result.Name != test.expected.Name {
			t.Errorf("Expected TLD name to be %s, but got %s for input %s", test.expected.Name, result.Name, test.name)
		}
	}
}

func TestTLD_SetUname(t *testing.T) {
	tests := []struct {
		name          string
		inputTLD      *TLD
		expectedUName string
	}{
		{
			name:          "example.com",
			inputTLD:      &TLD{Name: "example.com"},
			expectedUName: "",
		},
		{
			name:          "ünicode.com",
			inputTLD:      &TLD{Name: "xn--nicode-2ya.com"},
			expectedUName: "ünicode.com",
		},
	}

	for _, test := range tests {
		test.inputTLD.SetUname()
		if test.inputTLD.UName.String() != test.expectedUName {
			t.Errorf("Expected UName to be %s, but got %s for input %s", test.expectedUName, test.inputTLD.UName, test.name)
		}
	}
}

func TestTLD_SetTLDType(t *testing.T) {
	tests := []struct {
		name     string
		inputTLD *TLD
		expected TLDType
	}{
		{
			name:     "example.com",
			inputTLD: &TLD{Name: "example.com"},
			expected: TLDTypeSLD,
		},
		{
			name:     "co.uk",
			inputTLD: &TLD{Name: "co.uk"},
			expected: TLDTypeSLD,
		},
		{
			name:     "uk",
			inputTLD: &TLD{Name: "uk"},
			expected: TLDTypeCCTLD,
		},
		{
			name:     "org",
			inputTLD: &TLD{Name: "org"},
			expected: TLDTypeGTLD,
		},
	}

	for _, test := range tests {
		test.inputTLD.setTLDType()
		if test.inputTLD.Type != test.expected {
			t.Errorf("Expected TLD type to be %v, but got %v for input %s", test.expected, test.inputTLD.Type, test.name)
		}
	}
}

func TestTLDType_String(t *testing.T) {
	tests := []struct {
		name     string
		input    TLDType
		expected string
	}{
		{
			name:     "TLDTypeSLD",
			input:    TLDTypeSLD,
			expected: "second-level",
		},
		{
			name:     "TLDTypeCCTLD",
			input:    TLDTypeCCTLD,
			expected: "country-code",
		},
		{
			name:     "TLDTypeGTLD",
			input:    TLDTypeGTLD,
			expected: "generic",
		},
	}

	for _, test := range tests {
		result := test.input.String()
		if result != test.expected {
			t.Errorf("Expected String() to return %s, but got %s for input %s", test.expected, result, test.name)
		}
	}
}

func TestTLDTeste_AddPhase(t *testing.T) {
	tests := []struct {
		name     string
		inputTLD *TLD
		phase    *Phase
		err      error
	}{
		{
			name:     "example.com",
			inputTLD: &TLD{Name: "example.com"},
			phase:    &Phase{Name: "GA", Type: PhaseTypeGA},
			err:      nil,
		},
		{
			name:     "example.com",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}}},
			phase:    &Phase{Name: "GA", Type: PhaseTypeGA},
			err:      ErrPhaseAlreadyExists,
		},
		{
			name:     "example.com",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1)}}},
			phase:    &Phase{Name: "Launch", Type: PhaseTypeLaunch, Starts: time.Now()},
			err:      ErrPhaseOverlaps,
		},
	}

	for _, test := range tests {
		err := test.inputTLD.AddPhase(test.phase)
		if err != test.err {
			t.Errorf("Expected error to be %v, but got %v for input %s", test.err, err, test.name)
		}
		if err == nil {
			if len(test.inputTLD.Phases) == 0 {
				t.Errorf("Expected TLD to have a phase, but it has none for input %s", test.name)
			}
		}
	}
}

func TestTLD_GetCurrentPhase(t *testing.T) {
	endDateTime := time.Now().AddDate(0, 0, 1)
	tests := []struct {
		name     string
		inputTLD *TLD
		expected *Phase
		err      error
	}{
		{
			name:     "no phases",
			inputTLD: &TLD{Name: "example.com"},
			expected: nil,
			err:      ErrNoActivePhase,
		},
		{
			name:     "with end date",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1), Ends: &endDateTime}}},
			expected: &Phase{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1), Ends: &endDateTime},
			err:      nil,
		},
		{
			name:     "example.com",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1)}}},
			expected: &Phase{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1)},
			err:      nil,
		},
		{
			name:     "example.com",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, 1)}}},
			expected: nil,
			err:      ErrNoActivePhase,
		},
	}

	for _, test := range tests {
		result, err := test.inputTLD.GetCurrentPhase()
		if err != test.err {
			t.Errorf("Expected error to be %v, but got %v for input %s", test.err, err, test.name)
		}
		if result != nil && result.Name != test.expected.Name {
			t.Errorf("Expected phase name to be %s, but got %s for input %s", test.expected.Name, result.Name, test.name)
		}
	}
}

func TestTLD_FindPhaseByName(t *testing.T) {
	tests := []struct {
		name     string
		inputTLD *TLD
		phase    ClIDType
		expected *Phase
		err      error
	}{
		{
			name:     "example.com",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}}},
			phase:    "GA",
			expected: &Phase{Name: "GA", Type: PhaseTypeGA},
			err:      nil,
		},
		{
			name:     "example.com",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}}},
			phase:    "ga",
			expected: &Phase{Name: "GA", Type: PhaseTypeGA},
			err:      ErrPhaseNotFound,
		},
		{
			name:     "example.com",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}}},
			phase:    "Launch",
			expected: nil,
			err:      ErrPhaseNotFound,
		},
	}

	for _, test := range tests {
		result, err := test.inputTLD.FindPhaseByName(test.phase)
		if err != test.err {
			t.Errorf("Expected error to be %v, but got %v for input %s", test.err, err, test.name)
		}
		if result != nil && result.Name != test.expected.Name {
			t.Errorf("Expected phase name to be %s, but got %s for input %s", test.expected.Name, result.Name, test.name)
		}
	}
}

func TestTLD_DeletePhaseByName(t *testing.T) {
	endDate := time.Now().AddDate(0, 0, -1)
	tests := []struct {
		name     string
		inputTLD *TLD
		phase    ClIDType
		err      error
	}{
		{
			name:     "phase doesn't exist",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}}},
			phase:    "ga",
			err:      ErrPhaseNotFound,
		},
		{
			name:     "delete current phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1)}}},
			phase:    "GA",
			err:      ErrDeleteCurrentPhase,
		},
		{
			name:     "delete historic phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -2), Ends: &endDate}}},
			phase:    "GA",
			err:      ErrDeleteHistoricPhase,
		},
		{
			name:     "Successful delete",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, 1)}}},
			phase:    "GA",
			err:      nil,
		},
	}

	for _, test := range tests {
		err := test.inputTLD.DeletePhase(test.phase)
		if err != test.err {
			t.Errorf("Expected error to be %v, but got %v for input %s", test.err, err, test.name)
		}
		if err == nil {
			if len(test.inputTLD.Phases) != 0 {
				t.Errorf("Expected TLD to have no phases, but it has some for input %s", test.name)
			}
		}
	}
}
