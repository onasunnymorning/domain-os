package rest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

// CreateTestContext creates a new gin context for testing purposes
func CreateTestContext(path string) *gin.Context {
	pathSlice := strings.Split(path, "/")
	fmt.Println(pathSlice)
	// Create a new response recorder
	w := httptest.NewRecorder()

	// Create a new gin engine
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(w)

	// Set the request with the specified path
	req, _ := http.NewRequest(http.MethodPost, path, nil)
	c.Request = req

	// Set the id/name param
	identifier := pathSlice[len(pathSlice)-1]
	c.Params = []gin.Param{
		{
			Key:   "id",
			Value: identifier,
		},
		{
			Key:   "name",
			Value: identifier,
		},
	}
	c.Request.URL, _ = url.Parse(path)
	// c.Set("id", strings.Split(path, "/")[:len(strings.Split(path, "/"))-1])

	// Use unsafe to set the unexported fullPath field
	fullPathField := reflect.ValueOf(c).Elem().FieldByName("fullPath")
	reflect.NewAt(fullPathField.Type(), unsafe.Pointer(fullPathField.UnsafeAddr())).Elem().SetString(path)

	return c
}

func TestGetActionFromContext(t *testing.T) {
	tcases := []struct {
		name   string
		action string
	}{
		{"POST", entities.EventTypeCreate},
		{"GET", entities.EventTypeInfo},
		{"PUT", entities.EventTypeUpdate},
		{"DELETE", entities.EventTypeDelete},
		{"UNKNOWN", entities.EventTypeUnknown},
	}

	for _, tc := range tcases {
		t.Run("", func(t *testing.T) {
			c := &gin.Context{
				Request: &http.Request{
					Method: tc.name,
				},
			}

			action := GetActionFromContext(c)
			require.Equal(t, tc.action, action)
		})
	}
}

func TestGetObjectTypeFromContext(t *testing.T) {
	tcases := []struct {
		Name    string
		url     string
		objType string
	}{
		{"emptry", "", entities.ObjectTypeUnknown},
		{"contacts", "/contacts/id", entities.ObjectTypeContact},
		{"nndns", "/nndns", entities.ObjectTypeNNDN},
		{"unknown", "/unknown", entities.ObjectTypeUnknown},
	}

	for _, tc := range tcases {
		t.Run("", func(t *testing.T) {
			c := CreateTestContext(tc.url)

			objType := GetObjectTypeFromContext(c)
			require.Equal(t, tc.objType, objType)
		})
	}
}

// func TestPublishEvent(t *testing.T) {
// 	c := CreateTestContext("/contacts/clid01")
// 	expectedEvent := entities.NewEvent("AdminAPI", "admin", entities.EventTypeCreate, entities.ObjectTypeContact, "clid01", "/contacts/clid01")

// 	handler := PublishEvent(nil, "")
// 	handler(c)

// 	event, _ := c.Get("event")
// 	require.NotNil(t, event)

// 	e, ok := event.(*entities.Event)
// 	require.True(t, ok)

//		require.Equal(t, expectedEvent.Source, e.Source)
//		require.Equal(t, expectedEvent.User, e.User)
//		require.Equal(t, expectedEvent.Action, e.Action)
//		require.Equal(t, expectedEvent.ObjectType, e.ObjectType)
//		require.Equal(t, expectedEvent.ObjectID, e.ObjectID)
//		require.Equal(t, expectedEvent.EndPoint, e.EndPoint)
//	}
func TestGetObjectIDFromContext(t *testing.T) {
	tcases := []struct {
		Name       string
		URL        string
		ObjectType string
		ExpectedID string
	}{
		{"empty", "", entities.ObjectTypeUnknown, entities.ObjectIDUnknown},
		{"contacts", "/contacts/id", entities.ObjectTypeContact, "id"},
		{"nndns", "/nndns/name", entities.ObjectTypeNNDN, "name"},
		{"unknown", "/unknown", entities.ObjectTypeUnknown, entities.ObjectIDUnknown},
	}
	for _, tc := range tcases {
		t.Run(tc.Name, func(t *testing.T) {
			c := CreateTestContext(tc.URL)
			objectID := GetObjectIDFromContext(c)
			require.Equal(t, tc.ExpectedID, objectID)
		})
	}
}
func TestSetEventDetailsFromRequest(t *testing.T) {
	tcases := []struct {
		Name       string
		Action     string
		ObjectType string
		ObjectID   string
	}{
		{"set contact event details", entities.EventTypeCreate, entities.ObjectTypeContact, "contactID"},
		{"set nndn event details", entities.EventTypeUpdate, entities.ObjectTypeNNDN, "nndnName"},
		{"set unknown event details", entities.EventTypeUnknown, entities.ObjectTypeUnknown, entities.ObjectIDUnknown},
	}

	for _, tc := range tcases {
		t.Run(tc.Name, func(t *testing.T) {
			c := CreateTestContext("/test/path")
			event := entities.NewEvent("app", "user", "", "", "", "/test/path")
			c.Set("event", event)

			SetEventDetailsFromRequest(c, tc.Action, tc.ObjectType, tc.ObjectID)

			updatedEvent := GetEventFromContext(c)
			require.NotNil(t, updatedEvent)
			require.Equal(t, tc.Action, updatedEvent.Action)
			require.Equal(t, tc.ObjectType, updatedEvent.ObjectType)
			require.Equal(t, tc.ObjectID, updatedEvent.ObjectID)
		})
	}

	t.Run("event not found in context", func(t *testing.T) {
		c := CreateTestContext("/test/path")
		SetEventDetailsFromRequest(c, entities.EventTypeCreate, entities.ObjectTypeContact, "contactID")
		event := GetEventFromContext(c)
		require.Nil(t, event)
	})
}
func TestSetEvent(t *testing.T) {
	t.Run("set event in context", func(t *testing.T) {
		c := CreateTestContext("/test/path")
		event := entities.NewEvent("app", "user", entities.EventTypeCreate, entities.ObjectTypeContact, "contactID", "/test/path")

		SetEvent(c, event)

		retrievedEvent := GetEvent(c)
		require.NotNil(t, retrievedEvent)
		require.Equal(t, event, retrievedEvent)
	})
}

func TestGetEvent(t *testing.T) {
	t.Run("get event from context", func(t *testing.T) {
		c := CreateTestContext("/test/path")
		event := entities.NewEvent("app", "user", entities.EventTypeCreate, entities.ObjectTypeContact, "contactID", "/test/path")
		c.Set(string(eventCtxKey), event)

		retrievedEvent := GetEvent(c)
		require.NotNil(t, retrievedEvent)
		require.Equal(t, event, retrievedEvent)
	})

	t.Run("event not found in context", func(t *testing.T) {
		c := CreateTestContext("/test/path")

		retrievedEvent := GetEvent(c)
		require.Nil(t, retrievedEvent)
	})

	t.Run("event type assertion failure", func(t *testing.T) {
		c := CreateTestContext("/test/path")
		c.Set(string(eventCtxKey), "invalid event type")

		retrievedEvent := GetEvent(c)
		require.Nil(t, retrievedEvent)
	})
}
