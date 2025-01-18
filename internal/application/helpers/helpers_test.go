package helpers

import (
	"context"
	"testing"

	"github.com/likexian/gokit/assert"
)

func TestGetCorrelationID(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{
			name: "Correlation ID exists",
			ctx:  context.WithValue(context.Background(), "correlationID", "existingCorrelationID"),
			want: "existingCorrelationID",
		},
		{
			name: "Correlation ID does not exist",
			ctx:  context.Background(),
			want: "newCorrelationID-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getCorrelationID(tt.ctx)
			assert.Contains(t, got, tt.want)
		})
	}
}
