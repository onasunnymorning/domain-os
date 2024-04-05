package main

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func main() {
	tc := []struct {
		name       string
		thisStart  string
		thisEnd    string
		otherStart string
		otherEnd   string
		expected   bool
	}{
		{
			name:       "both no end date",
			thisStart:  "2021-01-01T00:00:00Z",
			thisEnd:    "",
			otherStart: "2021-01-01T00:00:00Z",
			otherEnd:   "",
			expected:   true,
		},
		{
			name:       "no end + start after",
			thisStart:  "2021-01-01T00:00:00Z",
			thisEnd:    "",
			otherStart: "2022-01-01T00:00:00Z",
			otherEnd:   "2123-01-01T00:00:00Z",
			expected:   true,
		},
	}

	for _, tt := range tc {
		// VARS
		var thisStart, thisEnd, otherStart, otherEnd time.Time
		var err error
		// SETUP
		thisStart, err = time.Parse(time.RFC3339, tt.thisStart)
		if err != nil {
			panic(err)
		}
		otherStart, err = time.Parse(time.RFC3339, tt.otherStart)
		if err != nil {
			panic(err)
		}
		// Create the two phases
		thisPhase, err := entities.NewPhase("thisPhase", "GA", thisStart)
		if err != nil {
			panic(err)
		}
		otherPhase, err := entities.NewPhase("otherPhase", "GA", otherStart)
		if err != nil {
			panic(err)
		}
		// Set the enddates if applicable
		if tt.thisEnd != "" {
			thisEnd, err = time.Parse(time.RFC3339, tt.thisEnd)
			if err != nil {
				panic(err)
			}
			err = thisPhase.SetEnd(thisEnd)
			if err != nil {
				panic(err)
			}
		}
		if tt.otherEnd != "" {
			otherEnd, err = time.Parse(time.RFC3339, tt.otherEnd)
			if err != nil {
				panic(err)
			}
			err = otherPhase.SetEnd(otherEnd)
			if err != nil {
				panic(err)
			}
		}

		// Run the test
		fmt.Printf("thisPhase: %s Starts: %v Ends: %v\n", thisPhase.Name, thisPhase.Starts, thisPhase.Ends)
		fmt.Printf("otherPhase: %s Starts: %v Ends: %v\n", otherPhase.Name, otherPhase.Starts, otherPhase.Ends)
		fmt.Printf("Result: %v\n", thisPhase.OverlapsWith(otherPhase))
	}
}
