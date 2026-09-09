package rdevalidate

// Code is a stable, machine-readable identifier for one validation check
// result. Codes are part of the persisted record and of operator reporting;
// never rename one — add a new one.
type Code string

const (
	// --- intake ---
	CodeIntakeArtifactMissing     Code = "INTAKE_ARTIFACT_MISSING"
	CodeIntakeLimitCompressedSize Code = "INTAKE_LIMIT_COMPRESSED_SIZE"

	// --- signature ---
	CodeSigMalformed    Code = "SIG_MALFORMED"
	CodeSigKeyUntrusted Code = "SIG_KEY_UNTRUSTED" // no active trusted key for the tenant/TLD matches the signer
	CodeSigInvalid      Code = "SIG_INVALID"       // a trusted key signed something else: the deposit was altered

	// --- decrypt ---
	CodeDecryptNotForServiceKey Code = "DECRYPT_NOT_FOR_SERVICE_KEY" // deposit was not encrypted to any service key
	CodeDecryptFailed           Code = "DECRYPT_FAILED"              // corrupt ciphertext, integrity (MDC) failure, or not encrypted at all
	CodeDecryptKeyUnavailable   Code = "DECRYPT_KEY_UNAVAILABLE"     // service-side: our keyring is empty -> ERROR, never DVFN

	// --- unpack ---
	CodeArchiveUnsupportedLayout Code = "ARCHIVE_UNSUPPORTED_LAYOUT"
	CodeArchiveUnsafeEntry       Code = "ARCHIVE_UNSAFE_ENTRY"
	CodeArchiveLimitFileCount    Code = "ARCHIVE_LIMIT_FILE_COUNT"
	CodeArchiveLimitUnpackedSize Code = "ARCHIVE_LIMIT_UNPACKED_SIZE"
	CodeArchiveLimitNesting      Code = "ARCHIVE_LIMIT_NESTING"
	CodeArchiveNoXML             Code = "ARCHIVE_NO_XML"

	// --- xml ---
	CodeXMLMalformed Code = "XML_MALFORMED"
	CodeXMLNoDeposit Code = "XML_NO_DEPOSIT"
	CodeXMLNoHeader  Code = "XML_NO_HEADER"

	// --- rde ---
	CodeRDEHeaderTLDMismatch     Code = "RDE_HEADER_TLD_MISMATCH"
	CodeRDECountMismatch         Code = "RDE_COUNT_MISMATCH"
	CodeRDEObjectDecodeError     Code = "RDE_OBJECT_DECODE_ERROR"
	CodeRDEObjectInvalid         Code = "RDE_OBJECT_INVALID"         // an RDE-required element is missing or unparsable
	CodeRDEObjectEntityRejected  Code = "RDE_OBJECT_ENTITY_REJECTED" // WARNING: the domain-os entity constructor rejected an otherwise well-formed object
	CodeRDERequiredObjectMissing Code = "RDE_REQUIRED_OBJECT_MISSING"

	// --- cross-cutting ---
	CodeValidationTimeout Code = "VALIDATION_TIMEOUT" // -> ERROR
	CodeInternal          Code = "INTERNAL_ERROR"     // -> ERROR
	CodeFindingsTruncated Code = "FINDINGS_TRUNCATED" // INFO: the findings list hit MaxFindings
	CodeInfoInnerSigned   Code = "INFO_INNER_SIGNATURE_PRESENT"
)

// Severity of a finding. Any ERROR makes the run FAIL; WARNING and INFO never do.
type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

// Stage names the pipeline step a finding belongs to, in execution order.
type Stage string

const (
	StageIntake    Stage = "intake"
	StageSignature Stage = "signature"
	StageDecrypt   Stage = "decrypt"
	StageUnpack    Stage = "unpack"
	StageXML       Stage = "xml"
	StageRDE       Stage = "rde"
)

// errorClassCodes are service-side conditions under which the run cannot
// honestly decide either way. They yield Outcome ERROR (no notification).
var errorClassCodes = map[Code]bool{
	CodeDecryptKeyUnavailable: true,
	CodeValidationTimeout:     true,
	CodeInternal:              true,
}

// IsErrorClass reports whether the code maps to Outcome ERROR rather than FAIL.
func (c Code) IsErrorClass() bool { return errorClassCodes[c] }
