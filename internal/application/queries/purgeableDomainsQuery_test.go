package queries

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/assert"
)

func TestNewPurgeableDomainsQuery(t *testing.T) {
	tests := []struct {
		name    string
		clid    string
		date    string
		tld     string
		wantErr bool
	}{
		{
			name:    "Valid inputs",
			clid:    "validClID",
			date:    "2023-01-01",
			tld:     "com",
			wantErr: false,
		},
		{
			name:    "Invalid date format",
			clid:    "validClID",
			date:    "01-01-2023",
			tld:     "com",
			wantErr: true,
		},
		{
			name:    "Invalid ClID",
			clid:    "invalidClIDBeCauseItsTooLong",
			date:    "2023-01-01",
			tld:     "com",
			wantErr: true,
		},
		{
			name:    "Invalid TLD",
			clid:    "validClID",
			date:    "2023-01-01",
			tld:     "in--validTLD",
			wantErr: true,
		},
		{
			name:    "Empty inputs",
			clid:    "",
			date:    "",
			tld:     "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := NewPurgeableDomainsQuery(tt.clid, tt.date, tt.tld)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, query)
				if tt.date != "" {
					expectedDate, _ := time.Parse("2006-01-02", tt.date)
					assert.Equal(t, expectedDate, query.After)
				}
				if tt.clid != "" {
					assert.Equal(t, entities.ClIDType(tt.clid), query.ClID)
				}
				if tt.tld != "" {
					assert.Equal(t, entities.DomainName(tt.tld), query.TLD)
				}
			}
		})
	}
}
