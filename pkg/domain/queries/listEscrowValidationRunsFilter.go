package queries

// ListEscrowValidationRunsFilter narrows a listing of escrow validation runs.
// Tenant scope is not a filter: it is a typed parameter on the repository
// method and is enforced in SQL (ADR-0006).
type ListEscrowValidationRunsFilter struct {
	// TLDEquals restricts to one TLD (normalised ASCII, no trailing dot).
	TLDEquals string
	// OutcomeEquals restricts to one outcome (RUNNING, PASS, FAIL, ERROR).
	OutcomeEquals string
}

// ToQueryParams converts the filter to a query string fragment.
func (f ListEscrowValidationRunsFilter) ToQueryParams() string {
	q := ""
	if f.TLDEquals != "" {
		q += "&tld=" + f.TLDEquals
	}
	if f.OutcomeEquals != "" {
		q += "&outcome=" + f.OutcomeEquals
	}
	return q
}

// ListEscrowDepositsFilter narrows a listing of escrow deposits.
type ListEscrowDepositsFilter struct {
	TLDEquals string
}

// ToQueryParams converts the filter to a query string fragment.
func (f ListEscrowDepositsFilter) ToQueryParams() string {
	if f.TLDEquals != "" {
		return "&tld=" + f.TLDEquals
	}
	return ""
}

// ListEscrowSanitizationRunsFilter narrows a listing of escrow sanitization
// runs. Tenant scope is a typed parameter, not a filter (ADR-0006).
type ListEscrowSanitizationRunsFilter struct {
	// TLDEquals restricts to one TLD (normalised ASCII, no trailing dot).
	TLDEquals string
	// OutcomeEquals restricts to one outcome (RUNNING, PASS, QUARANTINED, ERROR).
	OutcomeEquals string
	// SourceValidationRunIDEquals restricts to the derivatives of one source run.
	SourceValidationRunIDEquals string
}

// ToQueryParams converts the filter to a query string fragment.
func (f ListEscrowSanitizationRunsFilter) ToQueryParams() string {
	q := ""
	if f.TLDEquals != "" {
		q += "&tld=" + f.TLDEquals
	}
	if f.OutcomeEquals != "" {
		q += "&outcome=" + f.OutcomeEquals
	}
	if f.SourceValidationRunIDEquals != "" {
		q += "&sourceValidationRunId=" + f.SourceValidationRunIDEquals
	}
	return q
}
