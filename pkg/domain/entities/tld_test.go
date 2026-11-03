package entities

import (
	"errors"
	"testing"
	"time"
)

func TestNewTLD(t *testing.T) {
	tests := []struct {
		name     string
		ryID     string
		expected *TLD
		err      error
	}{
		{
			name:     "example.com",
			ryID:     "apex",
			expected: &TLD{Name: "example.com"},
			err:      nil,
		},
		{
			name:     "-invalid",
			ryID:     "apex",
			expected: nil,
			err:      ErrInvalidLabelDash,
		},
		{
			name:     "validdomain-invalid-ryid.com",
			ryID:     "thisisaverylongryidthatshouldnotbeaccepted",
			expected: nil,
			err:      ErrInvalidClIDType,
		},
	}

	for _, test := range tests {
		result, err := NewTLD(test.name, test.ryID)
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
	futureEnd := time.Now().UTC().AddDate(2, 0, 0)
	futureEndFar := time.Now().UTC().AddDate(4, 0, 0)
	futureEndShort := time.Now().UTC().AddDate(0, 3, 0)
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
			name:     "GA and Launch name colision",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "SameSame", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1)}}},
			phase:    &Phase{Name: "SameSame", Type: PhaseTypeLaunch, Starts: time.Now()},
			err:      ErrPhaseAlreadyExists,
		},
		{
			name:     "add overlap GA",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1)}}},
			phase:    &Phase{Name: "GA2", Type: PhaseTypeGA, Starts: time.Now()},
			err:      ErrGAPhaseOverlaps,
		},
		{
			name:     "add Launch, overlap GA",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1)}}},
			phase:    &Phase{Name: "Launch", Type: PhaseTypeLaunch, Starts: time.Now()},
			err:      nil,
		},
		{
			name:     "add GA, overlap Launch",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "Launch", Type: PhaseTypeLaunch, Starts: time.Now()}}},
			phase:    &Phase{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1)},
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
		{
			name: "future GA phase + new GA phase before that without enddate",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{
				{Name: "GA1", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(1, 0, 0), Ends: &futureEnd},
			}},
			phase: &Phase{Name: "GA0", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 1, 0)},
			err:   ErrGAPhaseOverlaps,
		},
		{
			name: "future GA phase + new GA phase before that with enddate",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{
				{Name: "GA1", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(1, 0, 0), Ends: &futureEnd},
			}},
			phase: &Phase{Name: "GA0", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 1, 0), Ends: &futureEndShort},
			err:   nil,
		},
		{
			name: "future GA phase + new GA phase after that with enddate",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{
				{Name: "GA1", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(1, 0, 0), Ends: &futureEnd},
			}},
			phase: &Phase{Name: "GA0", Type: PhaseTypeGA, Starts: futureEnd.AddDate(0, 0, 1), Ends: &futureEndFar},
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
			name:     "Update end historic phase",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().UTC().AddDate(0, 0, -1), Ends: &endDate}}},
			phase:    "GA",
			endTime:  time.Now().UTC().AddDate(0, 0, -1),
			err:      ErrEndDateInPast,
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
			err:     ErrGAPhaseOverlaps,
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
			name: "update will cause overlap but this should not return an error for Launch pahses",
			inputTLD: &TLD{Name: "example.com", Phases: []Phase{
				{Name: "PreviouslyEnded", Type: PhaseTypeLaunch, Starts: time.Now().UTC().AddDate(0, 0, -500), Ends: &pastEndDate},
				{Name: "Launch", Type: PhaseTypeLaunch, Starts: time.Now().UTC().AddDate(0, 0, -5), Ends: &endDate},
				{Name: "FutureOverlapping", Type: PhaseTypeLaunch, Starts: time.Now().UTC().AddDate(0, 0, 500)},
			}},
			phase:   "Launch",
			endTime: time.Now().UTC().AddDate(0, 0, 501),
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
			err:     ErrGAPhaseOverlaps,
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
			err:   ErrGAPhaseOverlaps,
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
			err:   ErrGAPhaseOverlaps,
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
		for _, p := range result {
			if p.Type != PhaseTypeLaunch {
				t.Errorf("GetCurrentLaunchPhases returned the %s phase %s for input %s", p.Type, p.Name, test.name)
			}
		}
	}

	// A GA phase listed before two launch phases: the old loop ran over the
	// first len(launch) phases of any type, so it counted the GA phase and
	// missed the last launch phase.
	gaFirst := &TLD{Name: "example.com", Phases: []Phase{
		{Name: "GA", Type: PhaseTypeGA, Starts: time.Now().AddDate(0, 0, -1)},
		{Name: "Launch", Type: PhaseTypeLaunch, Starts: time.Now().AddDate(0, 0, -1), Ends: &endDateTime},
		{Name: "Launch2", Type: PhaseTypeLaunch, Starts: time.Now().AddDate(0, 0, -1), Ends: &endDateTime},
	}}
	got := gaFirst.GetCurrentLaunchPhases()
	if len(got) != 2 || got[0].Name != "Launch" || got[1].Name != "Launch2" {
		t.Errorf("Expected the two launch phases, got %v", got)
	}
}

func TestTLD_ActiveLaunchPhases(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	past := now.AddDate(0, -1, 0)
	future := now.AddDate(0, 1, 0)
	launch := func(name string, mode LaunchMode, starts time.Time, ends *time.Time) Phase {
		return Phase{Name: ClIDType(name), Type: PhaseTypeLaunch, Starts: starts, Ends: ends, Launch: LaunchSettings{Mode: mode, Model: RegistrationModelFirstCome}}
	}
	tld := &TLD{Name: "example", Phases: []Phase{
		{Name: "ga", Type: PhaseTypeGA, Starts: past},
		launch("sunrise", LaunchModeSunrise, past.AddDate(0, -1, 0), &past), // ended
		launch("landrush", LaunchModeLandrush, past, &future),
		launch("landrush2", LaunchModeLandrush, past, nil),
		launch("claims", LaunchModeClaims, past, &future),
		launch("later", LaunchModeOpen, future, nil), // not started
	}}
	names := func(ps []Phase) []string {
		out := []string{}
		for _, p := range ps {
			out = append(out, p.Name.String())
		}
		return out
	}
	if got := names(tld.ActiveLaunchPhases("", now)); len(got) != 3 || got[0] != "landrush" || got[1] != "landrush2" || got[2] != "claims" {
		t.Errorf("all active launch phases: got %v", got)
	}
	if got := names(tld.ActiveLaunchPhases(LaunchModeLandrush, now)); len(got) != 2 {
		t.Errorf("landrush phases: got %v", got)
	}
	if got := tld.ActiveLaunchPhases(LaunchModeSunrise, now); len(got) != 0 {
		t.Errorf("ended sunrise must not be active: got %v", names(got))
	}
	if got := tld.ActiveLaunchPhases(LaunchModeOpen, now); len(got) != 0 {
		t.Errorf("future open phase must not be active: got %v", names(got))
	}
	if got := (&TLD{}).ActiveLaunchPhases("", now); got == nil || len(got) != 0 {
		t.Errorf("no phases is an empty slice, got %v", got)
	}
}

func TestTLD_GAPhaseAt(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(1, 0, 0)
	tld := &TLD{Name: "example", Phases: []Phase{
		{Name: "ga1", Type: PhaseTypeGA, Starts: start, Ends: &end},
		{Name: "ga2", Type: PhaseTypeGA, Starts: end},
		{Name: "landrush", Type: PhaseTypeLaunch, Starts: start},
	}}
	tests := []struct {
		name string
		now  time.Time
		want string
		err  error
	}{
		{"before any GA phase", start.Add(-time.Second), "", ErrNoActivePhase},
		{"at the start instant the phase is in force", start, "ga1", nil},
		{"within ga1", start.AddDate(0, 6, 0), "ga1", nil},
		{"at the roll the next phase is in force", end, "ga2", nil},
		{"after the roll", end.AddDate(1, 0, 0), "ga2", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tld.GAPhaseAt(tt.now)
			if tt.err != nil {
				if err != tt.err {
					t.Fatalf("want %v, got %v", tt.err, err)
				}
				return
			}
			if err != nil || got.Name.String() != tt.want {
				t.Fatalf("want %s, got %v (%v)", tt.want, got, err)
			}
		})
	}
}

func TestTLD_RegistrationContextAt(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	past := now.AddDate(0, -1, 0)
	future := now.AddDate(0, 1, 0)
	launch := func(name string, mode LaunchMode, starts time.Time, ends *time.Time) Phase {
		return Phase{Name: ClIDType(name), Type: PhaseTypeLaunch, Starts: starts, Ends: ends, Launch: LaunchSettings{Mode: mode, Model: RegistrationModelFirstCome}}
	}
	withGA := &TLD{Name: "example", Phases: []Phase{
		{Name: "ga", Type: PhaseTypeGA, Starts: past},
		launch("sunrise", LaunchModeSunrise, past.AddDate(0, -1, 0), &past), // ended
		launch("landrush", LaunchModeLandrush, past, &future),
		launch("open-a", LaunchModeOpen, past, &future),
		launch("open-b", LaunchModeOpen, past, nil),
		launch("later", LaunchModeCustom, future, nil), // not started
	}}
	noGA := &TLD{Name: "example", Phases: []Phase{launch("landrush", LaunchModeLandrush, past, &future)}}
	withClaims := &TLD{Name: "example", Phases: []Phase{
		{Name: "ga", Type: PhaseTypeGA, Starts: past},
		launch("claims", LaunchModeClaims, past, &future),
		launch("landrush", LaunchModeLandrush, past, &future),
	}}

	tests := []struct {
		name       string
		tld        *TLD
		ref        PhaseRef
		wantPhase  string
		wantClaims string
		err        error
	}{
		{"no ref is the GA phase", withGA, PhaseRef{}, "ga", "", nil},
		{"no ref without a GA phase", noGA, PhaseRef{}, "", "", ErrNoActivePhase},
		{"by name", withGA, PhaseRef{Name: "landrush"}, "landrush", "", nil},
		{"by name and matching mode", withGA, PhaseRef{Name: "landrush", Mode: LaunchModeLandrush}, "landrush", "", nil},
		{"by name with the wrong mode", withGA, PhaseRef{Name: "landrush", Mode: LaunchModeOpen}, "", "", ErrLaunchPhaseModeMismatch},
		{"unknown name", withGA, PhaseRef{Name: "nope"}, "", "", ErrPhaseNotFound},
		{"GA phase by name is refused", withGA, PhaseRef{Name: "ga"}, "", "", ErrPhaseNotLaunch},
		{"ended phase by name", withGA, PhaseRef{Name: "sunrise"}, "", "", ErrPhaseNotActive},
		{"future phase by name", withGA, PhaseRef{Name: "later"}, "", "", ErrPhaseNotActive},
		{"by mode when one is open", withGA, PhaseRef{Mode: LaunchModeLandrush}, "landrush", "", nil},
		{"by mode when none is open", withGA, PhaseRef{Mode: LaunchModeSunrise}, "", "", ErrPhaseNotActive},
		{"by mode when two are open", withGA, PhaseRef{Mode: LaunchModeOpen}, "", "", ErrLaunchPhaseAmbiguous},
		{"ambiguity resolved by name", withGA, PhaseRef{Name: "open-b", Mode: LaunchModeOpen}, "open-b", "", nil},
		{"claims overlay on GA", withClaims, PhaseRef{}, "ga", "claims", nil},
		{"claims overlay on a launch phase", withClaims, PhaseRef{Mode: LaunchModeLandrush}, "landrush", "claims", nil},
		{"the claims phase itself", withClaims, PhaseRef{Mode: LaunchModeClaims}, "claims", "claims", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc, err := tt.tld.RegistrationContextAt(tt.ref, now)
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("want %v, got %v", tt.err, err)
				}
				if rc.Phase != nil {
					t.Fatalf("a refusal carries no phase, got %v", rc.Phase.Name)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if rc.Phase == nil || rc.Phase.Name.String() != tt.wantPhase {
				t.Fatalf("phase: want %s, got %v", tt.wantPhase, rc.Phase)
			}
			switch {
			case tt.wantClaims == "" && rc.Claims != nil:
				t.Fatalf("no claims phase expected, got %s", rc.Claims.Name)
			case tt.wantClaims != "" && (rc.Claims == nil || rc.Claims.Name.String() != tt.wantClaims):
				t.Fatalf("claims: want %s, got %v", tt.wantClaims, rc.Claims)
			}
		})
	}

	// The context holds copies: changing it does not change the TLD.
	rc, err := withGA.RegistrationContextAt(PhaseRef{Name: "landrush"}, now)
	if err != nil {
		t.Fatal(err)
	}
	rc.Phase.Name = "changed"
	if p, _ := withGA.FindPhaseByName("landrush"); p == nil {
		t.Error("the TLD's phase was renamed through the context")
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
