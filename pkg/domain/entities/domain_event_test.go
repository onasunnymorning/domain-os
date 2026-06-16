package entities

import (
	"testing"
)

func TestDomainEvent(t *testing.T) {
	data := "some test data"
	event := NewDomainEvent("test-source", "test.event", "test-subject", data)

	if event.ID == "" {
		t.Error("expected event ID to be generated, got empty string")
	}
	if event.Source != "test-source" {
		t.Errorf("expected source to be 'test-source', got %s", event.Source)
	}
	if event.Type != "test.event" {
		t.Errorf("expected type to be 'test.event', got %s", event.Type)
	}
	if event.Subject != "test-subject" {
		t.Errorf("expected subject to be 'test-subject', got %s", event.Subject)
	}
	if event.Time.IsZero() {
		t.Error("expected time to be set, got zero time")
	}
	if event.Data != data {
		t.Errorf("expected data to be '%v', got %v", data, event.Data)
	}
}
