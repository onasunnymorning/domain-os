package entities

import (
	"testing"
)

func TestNewRegistrarMapping(t *testing.T) {
	registrarMapping := NewRegistrarMapping()

	if registrarMapping == nil {
		t.Errorf("Expected non-nil RegistrarMapping, got nil")
	}

	if len(registrarMapping) != 0 {
		t.Errorf("Expected empty RegistrarMapping, got length %d", len(registrarMapping))
	}
}
