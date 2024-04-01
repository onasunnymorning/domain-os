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
			name:     "add first phase",
			inputTLD: &TLD{Name: "example.com"},
			phase:    &Phase{Name: "GA", Type: PhaseTypeGA},
			err:      nil,
		},
		{
			name:     "name colision GA",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}}},
			phase:    &Phase{Name: "GA", Type: PhaseTypeGA},
			err:      ErrPhaseAlreadyExists,
		},
		{
			name:     "name colision Launch",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "Launch", Type: PhaseTypeLaunch}}},
			phase:    &Phase{Name: "Launch", Type: PhaseTypeLaunch},
			err:      ErrPhaseAlreadyExists,
		},
		{
			name:     "add overlap GA",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1)}}},
			phase:    &Phase{Name: "GA2", Type: PhaseTypeGA, Starts: time.Now()},
			err:      ErrPhaseOverlaps,
		},
		{
			name:     "add Launch, overlap GA",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1)}}},
			phase:    &Phase{Name: "Launch", Type: PhaseTypeLaunch, Starts: time.Now()},
			err:      nil,
		},
		{
			name: "add Launch, double overlap GA",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{
				{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1)},
				{Name: "Launch", Type: PhaseTypeLaunch, Starts: time.Now()},
			}},
			phase: &Phase{Name: "Launch2", Type: PhaseTypeLaunch, Starts: time.Now()},
			err:   nil,
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
		result, err := test.inputTLD.GetCurrentGAPhase()
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

func TestTLD_EndPhase(t *testing.T) {
	endDate := time.Now().AddDate(0, 0, 2)
	pastEndDate := time.Now().AddDate(0, 0, -200)
	tests := []struct {
		name     string
		inputTLD *TLD
		phase    ClIDType
		endTime  time.Time
		err      error
	}{
		{
			name:     "phase doesn't exist",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}}},
			phase:    "ga",
			endTime:  time.Now().UTC().AddDate(0, 0, 1),
			err:      ErrPhaseNotFound,
		},
		{
			name:     "end date in the past",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -5)}}},
			phase:    "GA",
			endTime:  time.Now().UTC().AddDate(0, 0, -1),
			err:      ErrEndDateInPast,
		},
		{
			name:     "end date before start date",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, 5)}}},
			phase:    "GA",
			endTime:  time.Now().UTC().AddDate(0, 0, 4),
			err:      ErrEndDateBeforeStart,
		},
		{
			name:     "Successful end",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -1)}}},
			phase:    "GA",
			endTime:  time.Now().UTC().AddDate(0, 0, 1),
			err:      nil,
		},
		{
			name:     "Not UTC",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -1)}}},
			phase:    "GA",
			endTime:  time.Now().In(time.FixedZone("UTC+1", 3600)).AddDate(0, 0, 1),
			err:      ErrTimeStampNotUTC,
		},
		{
			name: "Successful end with ended historic phase and non-overlapping new phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{
				{Name: "PreviouslyEnded", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -500), Ends: &pastEndDate},
				{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -5), Ends: &endDate},
				{Name: "FutureNotOverlapping", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, 500)},
			}},
			phase:   "GA",
			endTime: time.Now().UTC().AddDate(0, 0, 1),
			err:     nil,
		},
		{
			name: "non UTC timestamp with ended historic phase and non-overlapping new phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{
				{Name: "PreviouslyEnded", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -500), Ends: &pastEndDate},
				{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -5), Ends: &endDate},
				{Name: "FutureNotOverlapping", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, 500)},
			}},
			phase:   "GA",
			endTime: time.Now().AddDate(0, 0, 1),
			err:     ErrTimeStampNotUTC,
		},
		{
			name: "update will cause overlap with existing phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{
				{Name: "PreviouslyEnded", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -500), Ends: &pastEndDate},
				{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -5), Ends: &endDate},
				{Name: "FutureOverlapping", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, 500)},
			}},
			phase:   "GA",
			endTime: time.Now().UTC().AddDate(0, 0, 501),
			err:     ErrPhaseOverlaps,
		},
	}

	for _, test := range tests {
		_, err := test.inputTLD.EndPhase(test.phase, test.endTime)
		if err != test.err {
			t.Errorf("Expected error to be %v, but got %v for input %s", test.err, err, test.name)
		}
	}
}

func TestTLD_CheckPhaseEndUpdate(t *testing.T) {
	endDate := time.Now().AddDate(0, 0, 2)
	pastEndDate := time.Now().AddDate(0, 0, -200)
	tests := []struct {
		name     string
		inputTLD *TLD
		phase    ClIDType
		endTime  time.Time
		err      error
	}{
		{
			name:     "phase doesn't exist",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}}},
			phase:    "ga",
			endTime:  time.Now().UTC().AddDate(0, 0, 1),
			err:      ErrPhaseNotFound,
		},
		{
			name:     "end date in the past",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -5)}}},
			phase:    "GA",
			endTime:  time.Now().UTC().AddDate(0, 0, -1),
			err:      ErrEndDateInPast,
		},
		{
			name:     "end date before start date",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, 5)}}},
			phase:    "GA",
			endTime:  time.Now().UTC().AddDate(0, 0, 4),
			err:      ErrEndDateBeforeStart,
		},
		{
			name:     "update historic phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -500), Ends: &pastEndDate}}},
			phase:    "GA",
			endTime:  time.Now().UTC().AddDate(0, 0, 1),
			err:      ErrUpdateHistoricPhase,
		},
		{
			name: "Successful end with ended historic phase and non-overlapping new phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{
				{Name: "PreviouslyEnded", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -500), Ends: &pastEndDate},
				{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -5), Ends: &endDate},
				{Name: "FutureNotOverlapping", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, 500)},
			}},
			phase:   "GA",
			endTime: time.Now().UTC().AddDate(0, 0, 1),
			err:     nil,
		},
		{
			name: "update will cause overlap with existing phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{
				{Name: "PreviouslyEnded", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -500), Ends: &pastEndDate},
				{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -5), Ends: &endDate},
				{Name: "FutureOverlapping", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, 500)},
			}},
			phase:   "GA",
			endTime: time.Now().UTC().AddDate(0, 0, 501),
			err:     ErrPhaseOverlaps,
		},
	}

	for _, test := range tests {
		err := test.inputTLD.checkPhaseEndUpdate(test.phase, test.endTime)
		if err != test.err {
			t.Errorf("Expected error to be %v, but got %v for input %s", test.err, err, test.name)
		}
	}
}

func TestTLD_CheckPhaseEndUnset(t *testing.T) {
	existingEndDatePast := time.Now().UTC().AddDate(0, 0, -100)
	existingEndDateFar := time.Now().UTC().AddDate(0, 0, 100)
	existingEndDateNear := time.Now().UTC().AddDate(0, 0, 10)
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
			name:     "unset non-existing end date",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -5)}}},
			phase:    "GA",
			err:      nil,
		},
		{
			name:     "successful removal of enddate",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -5), Ends: &existingEndDateFar}}},
			phase:    "GA",
			err:      nil,
		},
		{
			name:     "try changing the past",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -500), Ends: &existingEndDatePast}}},
			phase:    "GA",
			err:      ErrUpdateHistoricPhase,
		},
		{
			name: "removing end date will cause overlap",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{
				{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -5), Ends: &existingEndDateNear},
				{Name: "Future Phase", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, 50), Ends: &existingEndDateFar},
			}},
			phase: "GA",
			err:   ErrPhaseOverlaps,
		},
	}

	for _, test := range tests {
		err := test.inputTLD.checkPhaseEndUnset(test.phase)
		if err != test.err {
			t.Errorf("Expected error to be %v, but got %v for input %s", test.err, err, test.name)
		}
	}
}

func TestTLD_UnSetPhaseEnd(t *testing.T) {
	existingEndDatePast := time.Now().UTC().AddDate(0, 0, -100)
	existingEndDateFar := time.Now().UTC().AddDate(0, 0, 100)
	existingEndDateNear := time.Now().UTC().AddDate(0, 0, 10)
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
			name:     "unset non-existing end date",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -5)}}},
			phase:    "GA",
			err:      nil,
		},
		{
			name:     "successful removal of enddate",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -5), Ends: &existingEndDateFar}}},
			phase:    "GA",
			err:      nil,
		},
		{
			name:     "try changing the past",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -500), Ends: &existingEndDatePast}}},
			phase:    "GA",
			err:      ErrUpdateHistoricPhase,
		},
		{
			name: "removing end date will cause overlap",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{
				{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -5), Ends: &existingEndDateNear},
				{Name: "Future Phase", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, 50), Ends: &existingEndDateFar},
			}},
			phase: "GA",
			err:   ErrPhaseOverlaps,
		},
	}

	for _, test := range tests {
		_, err := test.inputTLD.UnSetPhaseEnd(test.phase)
		if err != test.err {
			t.Errorf("Expected error to be %v, but got %v for input %s", test.err, err, test.name)
		}
	}

}

func TestTLD_GetGAPhases(t *testing.T) {
	tests := []struct {
		name     string
		inputTLD *TLD
		expected int
	}{
		{
			name:     "no phases",
			inputTLD: &TLD{Name: "example.com"},
			expected: 0,
		},
		{
			name:     "one GA phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}}},
			expected: 1,
		},
		{
			name:     "one GA and one Launch phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}, {Name: "Launch", Type: PhaseTypeLaunch}}},
			expected: 1,
		},
		{
			name:     "two GA phases",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}, {Name: "GA2", Type: PhaseTypeGA}}},
			expected: 2,
		},
	}

	for _, test := range tests {
		result := test.inputTLD.GetGAPhases()
		if len(result) != test.expected {
			t.Errorf("Expected number of GA phases to be %d, but got %d for input %s", test.expected, len(result), test.name)
		}
	}
}

func TestTLD_CheckPhaseNameExists(t *testing.T) {
	tests := []struct {
		name     string
		inputTLD *TLD
		phase    ClIDType
		expected error
	}{
		{
			name:     "phase doesn't exist",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}}},
			phase:    "ga",
			expected: nil,
		},
		{
			name:     "phase exists",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}}},
			phase:    "GA",
			expected: ErrPhaseAlreadyExists,
		},
	}

	for _, test := range tests {
		result := test.inputTLD.checkPhaseNameExists(test.phase)
		if result != test.expected {
			t.Errorf("Expected result to be %v, but got %v for input %s", test.expected, result, test.name)
		}
	}
}

func TestTLD_GetLaunchPhases(t *testing.T) {
	tests := []struct {
		name     string
		inputTLD *TLD
		expected int
	}{
		{
			name:     "no phases",
			inputTLD: &TLD{Name: "example.com"},
			expected: 0,
		},
		{
			name:     "one Launch phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "Launch", Type: PhaseTypeLaunch}}},
			expected: 1,
		},
		{
			name:     "one GA and one Launch phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}, {Name: "Launch", Type: PhaseTypeLaunch}}},
			expected: 1,
		},
		{
			name:     "two Launch phases",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "Launch", Type: PhaseTypeLaunch}, {Name: "Launch2", Type: PhaseTypeLaunch}}},
			expected: 2,
		},
	}

	for _, test := range tests {
		result := test.inputTLD.GetLaunchPhases()
		if len(result) != test.expected {
			t.Errorf("Expected number of Launch phases to be %d, but got %d for input %s", test.expected, len(result), test.name)
		}
	}
}

func TestTLD_GetCurrentLaunchPhases(t *testing.T) {
	endDateTime := time.Now().AddDate(0, 0, 1)
	tests := []struct {
		name     string
		inputTLD *TLD
		expected int
	}{
		{
			name:     "no phases",
			inputTLD: &TLD{Name: "example.com"},
			expected: 0,
		},
		{
			name:     "one Launch phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "Launch", Type: PhaseTypeLaunch, Starts: time.Now().AddDate(0, 0, -1), Ends: &endDateTime}}},
			expected: 1,
		},
		{
			name:     "one GA and one Launch phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}, {Name: "Launch", Type: PhaseTypeLaunch, Starts: time.Now().AddDate(0, 0, -1), Ends: &endDateTime}}},
			expected: 1,
		},
		{
			name:     "two Launch phases",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "Launch", Type: PhaseTypeLaunch, Starts: time.Now().AddDate(0, 0, -1), Ends: &endDateTime}, {Name: "Launch2", Type: PhaseTypeLaunch, Starts: time.Now().AddDate(0, 0, -1), Ends: &endDateTime}}},
			expected: 2,
		},
	}

	for _, test := range tests {
		result := test.inputTLD.GetCurrentLaunchPhases()
		if len(result) != test.expected {
			t.Errorf("Expected number of Launch phases to be %d, but got %d for input %s", test.expected, len(result), test.name)
		}
	}
}

func TestTLD_GetCurrentPhases(t *testing.T) {
	endDateTime := time.Now().AddDate(0, 0, 1)
	tests := []struct {
		name     string
		inputTLD *TLD
		expected int
	}{
		{
			name:     "no phases",
			inputTLD: &TLD{Name: "example.com"},
			expected: 0,
		},
		{
			name:     "one GA and one Launch phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA}, {Name: "Launch", Type: PhaseTypeLaunch, Starts: time.Now().AddDate(0, 0, -1), Ends: &endDateTime}}},
			expected: 2,
		},
		{
			name:     "two Launch phases",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "Launch", Type: PhaseTypeLaunch, Starts: time.Now().AddDate(0, 0, -1), Ends: &endDateTime}, {Name: "Launch2", Type: PhaseTypeLaunch, Starts: time.Now().AddDate(0, 0, -1), Ends: &endDateTime}}},
			expected: 2,
		},
	}

	for _, test := range tests {
		result := test.inputTLD.GetCurrentPhases()
		if len(result) != test.expected {
			t.Errorf("Expected number of Launch phases to be %d, but got %d for input %s", test.expected, len(result), test.name)
		}
	}
}
