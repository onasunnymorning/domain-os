package queries

import (
	"testing"
	"time"
)

func TestToQueryParams_EmptyFilter(t *testing.T) {
	filter := ListDomainsFilter{}
	got := filter.ToQueryParams()
	if got != "" {
		t.Errorf("expected empty query string, got: %q", got)
	}
}

func TestToQueryParams_FullFilter(t *testing.T) {
	// Prepare sample times
	expiresBefore := time.Date(2023, 10, 10, 12, 0, 0, 0, time.UTC)
	expiresAfter := time.Date(2023, 11, 10, 12, 0, 0, 0, time.UTC)
	createdBefore := time.Date(2022, 10, 10, 12, 0, 0, 0, time.UTC)
	createdAfter := time.Date(2022, 11, 10, 12, 0, 0, 0, time.UTC)

	filter := ListDomainsFilter{
		RoidGreaterThan: "123",
		NameLike:        "example",
		NameEquals:      "example.com",
		TldEquals:       "com",
		ClidEquals:      "clid_abc",
		ExpiresBefore:   expiresBefore,
		ExpiresAfter:    expiresAfter,
		CreatedBefore:   createdBefore,
		CreatedAfter:    createdAfter,
	}
	got := filter.ToQueryParams()

	expected := "&roid_greater_than=123" +
		"&name_like=example" +
		"&name_equals=example.com" +
		"&tld_equals=com" +
		"&clid_equals=clid_abc" +
		"&expires_before=" + expiresBefore.Format(time.RFC3339) +
		"&expires_after=" + expiresAfter.Format(time.RFC3339) +
		"&created_before=" + createdBefore.Format(time.RFC3339) +
		"&created_after=" + createdAfter.Format(time.RFC3339)

	if got != expected {
		t.Errorf("expected query string:\n%q\ngot:\n%q", expected, got)
	}
}

func TestToQueryParams_PartialFilter(t *testing.T) {
	// Prepare sample time
	createdAfter := time.Date(2022, 11, 10, 12, 0, 0, 0, time.UTC)

	filter := ListDomainsFilter{
		NameEquals:   "example.net",
		CreatedAfter: createdAfter,
	}
	got := filter.ToQueryParams()

	expected := "&name_equals=example.net" +
		"&created_after=" + createdAfter.Format(time.RFC3339)

	if got != expected {
		t.Errorf("expected query string:\n%q\ngot:\n%q", expected, got)
	}
}
