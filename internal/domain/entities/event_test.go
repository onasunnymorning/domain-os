package entities

import "testing"

func TestNewEvent(t *testing.T) {
	app := "AdminAPI"
	actor := "John Doe"
	action := "Create"
	objectType := "Contact"
	objectID := "12345"
	endPoint := "/api/contacts"

	event := NewEvent(app, actor, action, objectType, objectID, endPoint)

	if event.Source != app {
		t.Errorf("expected app %s, got %s", app, event.Source)
	}
	if event.User != actor {
		t.Errorf("expected actor %s, got %s", actor, event.User)
	}
	if event.Action != action {
		t.Errorf("expected action %s, got %s", action, event.Action)
	}
	if event.ObjectType != objectType {
		t.Errorf("expected object type %s, got %s", objectType, event.ObjectType)
	}
	if event.ObjectID != objectID {
		t.Errorf("expected object ID %s, got %s", objectID, event.ObjectID)
	}
	if event.EndPoint != endPoint {
		t.Errorf("expected end point %s, got %s", endPoint, event.EndPoint)
	}
	if event.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}
func TestEvent_ToJSONBytes(t *testing.T) {
	event := NewEvent(AppAdminAPI, "John Doe", "Create", ObjectTypeContact, "12345", "/api/contacts")
	jsonBytes := event.ToJSONBytes()

	if jsonBytes == nil {
		t.Error("expected non-nil JSON bytes")
	}

	// Add your assertions here
}
