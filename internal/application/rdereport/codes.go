package rdereport

import "github.com/onasunnymorning/domain-os/internal/application/rdevalidate"

// The DVFN <results> element carries <iirdea:result> entries whose "code"
// attribute is a four-digit number that "MUST be defined for the
// corresponding process" (draft-26 §1.4.1). The draft publishes ICANN's
// response codes to a notification (1000, 2xxx) but leaves the verification
// result vocabulary to the Data Escrow Agent, so this table is ours: the 4xxx
// range, grouped by pipeline stage. It is part of the emitted document and
// therefore stable — add codes, never renumber.
type resultCode struct {
	Code int
	Msg  string
}

var resultCodes = map[rdevalidate.Code]resultCode{
	rdevalidate.CodeIntakeArtifactMissing:     {4001, "Deposit or signature artifact was not received."},
	rdevalidate.CodeIntakeLimitCompressedSize: {4002, "Deposit exceeds the accepted size."},

	rdevalidate.CodeSigMalformed:    {4101, "Detached signature could not be parsed."},
	rdevalidate.CodeSigKeyUntrusted: {4102, "Detached signature was not made by a key trusted for this TLD."},
	rdevalidate.CodeSigInvalid:      {4103, "Detached signature does not verify over the deposit."},

	rdevalidate.CodeDecryptNotForServiceKey: {4201, "Deposit is not encrypted to the escrow agent's key."},
	rdevalidate.CodeDecryptFailed:           {4202, "Deposit could not be decrypted or failed its integrity check."},

	rdevalidate.CodeArchiveUnsupportedLayout: {4301, "Deposit payload has an unsupported layout."},
	rdevalidate.CodeArchiveUnsafeEntry:       {4302, "Deposit archive contains an unsafe entry."},
	rdevalidate.CodeArchiveLimitFileCount:    {4303, "Deposit archive contains too many entries."},
	rdevalidate.CodeArchiveLimitUnpackedSize: {4304, "Deposit payload exceeds the unpacked size limit."},
	rdevalidate.CodeArchiveLimitNesting:      {4305, "Deposit payload exceeds the compression nesting limit."},
	rdevalidate.CodeArchiveNoXML:             {4306, "Deposit archive contains no XML payload."},

	rdevalidate.CodeXMLMalformed: {4401, "Deposit XML is not well-formed."},
	rdevalidate.CodeXMLNoDeposit: {4402, "Deposit XML has no rde:deposit element."},
	rdevalidate.CodeXMLNoHeader:  {4403, "Deposit XML has no rdeHeader:header element."},

	rdevalidate.CodeRDEHeaderTLDMismatch:     {4501, "Header TLD does not match the TLD the deposit was submitted for."},
	rdevalidate.CodeRDECountMismatch:         {4502, "Header object count does not match the deposit contents."},
	rdevalidate.CodeRDEObjectDecodeError:     {4503, "An RDE object could not be decoded."},
	rdevalidate.CodeRDEObjectInvalid:         {4504, "An RDE object is missing a required element."},
	rdevalidate.CodeRDERequiredObjectMissing: {4505, "Objects declared in the header are absent from the deposit."},
	// Reserved. Both are WARNING today and so never reach a DVFN, but the table
	// is append-only and renumbering later is not allowed, so they take their
	// slot in the RDE range now.
	rdevalidate.CodeRDEObjectNotReferenced:   {4506, "A contact or host in the deposit is referenced by no domain."},
	rdevalidate.CodeRDEReferenceNotInDeposit: {4507, "A domain references an object the deposit does not carry."},
}

// unknownResultCode is used for any ERROR-severity finding not in the table,
// so a new validator code can never produce a schema-invalid notification.
var unknownResultCode = resultCode{4999, "Deposit failed verification."}

// ResultCodeFor maps a validator code to its iirdea result code and message.
func ResultCodeFor(c rdevalidate.Code) (int, string) {
	if rc, ok := resultCodes[c]; ok {
		return rc.Code, rc.Msg
	}
	return unknownResultCode.Code, unknownResultCode.Msg
}
