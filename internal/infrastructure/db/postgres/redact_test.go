package postgres

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "Tr@nsferS3cret!"

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{
			name: "top-level field",
			in:   map[string]interface{}{"Name": "example.com", "AuthInfo": testSecret},
			want: `{"Name":"example.com","AuthInfo":"[redacted]"}`,
		},
		{
			name: "key spellings",
			in:   map[string]interface{}{"authInfo": testSecret, "auth_info": testSecret, "AUTH-INFO": testSecret},
			want: `{"authInfo":"[redacted]","auth_info":"[redacted]","AUTH-INFO":"[redacted]"}`,
		},
		{
			name: "nested in objects and arrays",
			in: map[string]interface{}{
				"Domains": []interface{}{
					map[string]interface{}{"AuthInfo": testSecret},
					map[string]interface{}{"Inner": map[string]interface{}{"AuthInfo": testSecret}},
				},
			},
			want: `{"Domains":[{"AuthInfo":"[redacted]"},{"Inner":{"AuthInfo":"[redacted]"}}]}`,
		},
		{
			name: "non-string value",
			in:   map[string]interface{}{"AuthInfo": map[string]string{"pw": testSecret}},
			want: `{"AuthInfo":"[redacted]"}`,
		},
		{
			name: "empty and null left alone",
			in:   map[string]interface{}{"AuthInfo": "", "Other": map[string]interface{}{"AuthInfo": nil}},
			want: `{"AuthInfo":"","Other":{"AuthInfo":null}}`,
		},
		{
			name: "unrelated keys untouched, large integers exact",
			in:   map[string]interface{}{"AuthInfoHint": "x", "ID": int64(1<<62 + 1)},
			want: `{"AuthInfoHint":"x","ID":4611686018427387905}`,
		},
		{
			name: "nil",
			in:   nil,
			want: `null`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := redactSecrets(tc.in)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
			assert.NotContains(t, string(got), testSecret)
		})
	}
}

// TestToDBDomainEvent_RedactsAuthInfo feeds ToDBDomainEvent the payloads the
// domain and contact producers actually publish and checks that no column of
// the resulting row carries the authInfo.
func TestToDBDomainEvent_RedactsAuthInfo(t *testing.T) {
	domain := entities.Domain{Name: "example.com", AuthInfo: testSecret}
	contact := entities.Contact{ID: "c1", AuthInfo: testSecret}

	tests := []struct {
		name                   string
		command, before, after interface{}
	}{
		{"domain.registered", commands.RegisterDomainCommand{Name: "example.com", AuthInfo: testSecret}, nil, &domain},
		{"domain.admin_created", &commands.CreateDomainCommand{Name: "example.com", AuthInfo: testSecret}, nil, &domain},
		{"domain.updated", &commands.UpdateDomainCommand{AuthInfo: testSecret}, &domain, &domain},
		{"domain.admin_deleted", nil, &domain, nil},
		{"contact.created", &commands.CreateContactCommand{ID: "c1", AuthInfo: testSecret}, nil, &contact},
		{"contact.bulk_created", []*commands.CreateContactCommand{{AuthInfo: testSecret}, {AuthInfo: testSecret}}, nil, []*entities.Contact{&contact, &contact}},
		{"contact.updated", &commands.UpdateContactCommand{AuthInfo: testSecret}, &contact, &contact},
		{"contact.deleted", nil, &contact, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := entities.NewDomainEvent("test", tc.name, "subject", "msg", map[string]string{"k": "v"})
			e.Command, e.BeforeState, e.AfterState = tc.command, tc.before, tc.after

			rec, err := ToDBDomainEvent(e)
			require.NoError(t, err)

			for col, b := range map[string][]byte{
				"data": rec.Data, "command": rec.Command, "before_state": rec.BeforeState, "after_state": rec.AfterState,
			} {
				assert.False(t, bytes.Contains(b, []byte(testSecret)), "%s column contains the authInfo", col)
			}
			for col, b := range map[string][]byte{
				"command": rec.Command, "before_state": rec.BeforeState, "after_state": rec.AfterState,
			} {
				if b != nil {
					assert.Contains(t, string(b), `"AuthInfo":"`+RedactedMarker+`"`, "%s column lost the AuthInfo marker", col)
				}
			}

			// The row still decodes for the relay and the events API.
			_, err = rec.ToDomainEvent()
			require.NoError(t, err)
		})
	}
}

func TestFromTombstone_RedactsAuthInfo(t *testing.T) {
	rec := fromTombstone(&entities.DomainTombstone{
		LastSnapshot: &entities.Domain{Name: "example.com", AuthInfo: testSecret},
	})
	assert.NotContains(t, string(rec.LastSnapshot), testSecret)

	var snap map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.LastSnapshot, &snap))
	assert.Equal(t, RedactedMarker, snap["AuthInfo"])
	assert.Equal(t, "example.com", snap["Name"])
}
