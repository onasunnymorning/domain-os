package services

import (
	"context"
	"testing"
)

func TestDnssecServiceRegex(t *testing.T) {
	service := NewDnssecService()
	ctx := context.Background()

	invalidDomains := []string{
		"example.com; rm -rf /",
		"google.com & echo 'hacked'",
		"space domain.com",
		"test@domain.com",
		"domain",
		"-domain.com",
		"domain-.com",
		"domain.c", // TLD must be >= 2 chars
	}

	for _, d := range invalidDomains {
		_, err := service.Visualize(ctx, d)
		if err == nil || err.Error() != "invalid domain format" {
			t.Errorf("Expected 'invalid domain format' for domain %q, got: %v", d, err)
		}
	}

	// For valid domains, dnsviz will fail gracefully if it's not installed in the test environment,
	// but the regex should pass and return a different error.
	validDomains := []string{
		"example.com",
		"a.b.c.co.uk",
		"domain-test.net",
		"123-num.org",
	}

	for _, d := range validDomains {
		_, err := service.Visualize(ctx, d)
		// We expect either dnsviz not found or a dnsviz execution error, NOT invalid format.
		if err != nil && err.Error() == "invalid domain format" {
			t.Errorf("Domain %q should be valid, but got: %v", d, err)
		}
	}
}
