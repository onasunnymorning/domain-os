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

func TestRemoveNonASCII(t *testing.T) {
	input := "Hello, 世界!"
	expected := "Hello, !"
	result := RemoveNonASCII(input)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}

func TestRemoveNonAlphaNumeric(t *testing.T) {
	input := "Hello, -_!"
	expected := "Hello-"
	result := RemoveNonAlphaNumeric(input)
	if result != expected {
		t.Errorf("Expected %s, but got %s", expected, result)
	}
}
