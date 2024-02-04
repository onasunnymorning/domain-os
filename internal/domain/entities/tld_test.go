package entities

import "testing"

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
			expectedUName: "example.com",
		},
		{
			name:          "ünicode.com",
			inputTLD:      &TLD{Name: "ünicode.com"},
			expectedUName: "ünicode.com",
		},
	}

	for _, test := range tests {
		test.inputTLD.SetUname()
		if test.inputTLD.UName != test.expectedUName {
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
