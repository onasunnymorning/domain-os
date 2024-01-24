package entities

import (
	"testing"
)

func TestIsASCII(t *testing.T) {
	ascii := "ascii"
	nonASCII := "ñ"
	if !IsASCII(ascii) {
		t.Errorf("Expected %s to be ASCII", ascii)
	}
	if IsASCII(nonASCII) {
		t.Errorf("Expected %s to not be ASCII", nonASCII)
	}
}
