package response

import (
	"testing"
	"time"
)

func TestNewIsAccreditedResponse(t *testing.T) {
	registrarClID := "exampleRegistrar"
	tldName := "com"
	resp := NewIsAccreditedResponse(registrarClID, tldName)

	if resp.RegistrarClID != registrarClID {
		t.Errorf("RegistrarClID: want %q, got %q", registrarClID, resp.RegistrarClID)
	}
	if resp.TLDName != tldName {
		t.Errorf("TLDName: want %q, got %q", tldName, resp.TLDName)
	}
	if resp.IsAccredited {
		t.Error("IsAccredited should be false by default")
	}
	if time.Since(resp.Timestamp) > time.Second {
		t.Errorf("Timestamp too old: got %v", resp.Timestamp)
	}
}
