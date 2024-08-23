package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRDEReport(t *testing.T) {
	id := "123"
	resend := 1
	crDate := time.Now()
	watermark := time.Now()
	header := RDEHeader{
		// Initialize the RDEHeader fields here
	}

	report := NewRDEReport(id, resend, crDate, watermark, header)

	assert.NotNil(t, report)
	assert.Equal(t, "urn:ietf:params:xml:ns:rdeReport-1.0", report.XMLNS)
	assert.Equal(t, id, report.ID)
	assert.Equal(t, 1, report.Version)
	assert.Equal(t, RYDE_ESCROW_RFC, report.RydeSpecEscrow)
	assert.Equal(t, RYDE_MAPPING_RFC, report.RydeSpecMapping)
	assert.Equal(t, resend, report.Resend)
	assert.Equal(t, crDate, report.CrDate)
	assert.Equal(t, RDEReportTypeFULL, report.Kind)
	assert.Equal(t, watermark, report.Watermark)
	assert.Equal(t, header, report.RdeHeader)
}
