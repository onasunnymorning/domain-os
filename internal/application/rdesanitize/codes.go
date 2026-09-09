package rdesanitize

// Code is a stable, machine-readable reason code. Codes are part of the record
// and of every operator-facing report, so they are never renumbered or reused.
type Code string

const (
	// source: the run cannot start from this validation run.
	CodeSourceNotAccepted     Code = "SRC_NOT_ACCEPTED"
	CodeSourceProfileUnknown  Code = "SRC_PROFILE_UNSUPPORTED"
	CodeSourceArtifactChanged Code = "SRC_ARTIFACT_CHANGED"

	// xml: the source document cannot be parsed safely.
	CodeXMLMalformed           Code = "XML_MALFORMED"
	CodeXMLDTDPresent          Code = "XML_DTD_PRESENT"
	CodeXMLUnsupportedEncoding Code = "XML_UNSUPPORTED_ENCODING"

	// limits: the source is within the validator's limits but outside ours.
	CodeLimitXMLDepth     Code = "LIMIT_XML_DEPTH"
	CodeLimitElementCount Code = "LIMIT_ELEMENT_COUNT"
	CodeLimitFieldBytes   Code = "LIMIT_FIELD_BYTES"
	CodeLimitUnpackedSize Code = "LIMIT_UNPACKED_SIZE"

	// policy: the profile does not classify something, so nothing is written.
	CodePolicyUnknownNamespace Code = "POLICY_UNKNOWN_NAMESPACE"
	CodePolicyUnknownElement   Code = "POLICY_UNKNOWN_ELEMENT"
	CodePolicyUnknownAttribute Code = "POLICY_UNKNOWN_ATTRIBUTE"

	// rewrite: a transformation could not be applied faithfully.
	CodeFQDNUnrewritable Code = "FQDN_UNREWRITABLE"

	// verification of what was produced.
	CodeDerivativeInvalid     Code = "DERIVATIVE_INVALID"
	CodeDerivativePIIDetected Code = "DERIVATIVE_PII_DETECTED"

	// cross-cutting, error class: the service could not decide.
	// #nosec G101 -- a reason code naming an unavailable key, not a key
	CodeTokenKeyUnavailable Code = "TOKEN_KEY_UNAVAILABLE"
	CodeSanitizeTimeout     Code = "SANITIZE_TIMEOUT"
	CodeInternal            Code = "INTERNAL_ERROR"

	// informational.
	CodeFindingsTruncated Code = "FINDINGS_TRUNCATED"
)

// Severity of a finding.
type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

// Stage names the phase that produced a finding.
type Stage string

const (
	StageSource  Stage = "source"
	StageRewrite Stage = "rewrite"
	StageVerify  Stage = "verify"
)

// errorClassCodes are conditions where *this service* could not decide, as
// opposed to a source it decided not to transform. The distinction matters:
// quarantining a deposit is a statement about the deposit, and must not be made
// because our own key store is down.
var errorClassCodes = map[Code]bool{
	CodeTokenKeyUnavailable: true,
	CodeSanitizeTimeout:     true,
	CodeInternal:            true,
}

// IsErrorClass reports whether the code means the service failed rather than
// the source being unfit to transform.
func (c Code) IsErrorClass() bool { return errorClassCodes[c] }
