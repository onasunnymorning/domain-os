package entities

import (
	"github.com/google/uuid"
)

// EscrowKeySelection is the set of key versions a run is allowed to use,
// resolved once at execution time and carried through workflow history
// (ADR-0009 §7). It holds references and public data only — never material —
// so it is safe in history, and a retry uses exactly this selection instead of
// whatever "the latest key" has become. Before use, every version is re-checked
// so that a revocation still applies to a run already in flight.
type EscrowKeySelection struct {
	Arrangement EffectiveEscrowArrangement `json:"arrangement"`
	// Verify are the depositor's verification versions usable for this deposit.
	Verify []EscrowKeyVersionRef `json:"verify"`
	// Decrypt are the receiver's decryption versions usable for this deposit.
	Decrypt []EscrowKeyVersionRef `json:"decrypt"`
	// Pseudonymise is the receiver's active pseudonymisation version for a
	// derivative; nil means none is available.
	Pseudonymise *EscrowKeyVersionRef `json:"pseudonymise,omitempty"`
}

// Evidence is what the run records about the selection once the pipeline has
// said which fingerprints it actually used.
func (s *EscrowKeySelection) Evidence(signingFingerprint, decryptionFingerprint string) EscrowRunKeyEvidence {
	if s == nil {
		return EscrowRunKeyEvidence{}
	}
	ev := EscrowRunKeyEvidence{
		TLDArrangementRevision:      s.Arrangement.TLDRevision,
		OperatorArrangementRevision: s.Arrangement.OperatorRevision,
		PlatformArrangementRevision: s.Arrangement.PlatformRevision,
	}
	if s.Arrangement.Depositor != nil {
		id := s.Arrangement.Depositor.PartyID
		ev.DepositorPartyID = &id
	}
	if s.Arrangement.Receiver != nil {
		id := s.Arrangement.Receiver.PartyID
		ev.ReceiverPartyID = &id
	}
	for _, v := range s.Verify {
		ev.CandidateKeyVersionIDs = append(ev.CandidateKeyVersionIDs, v.ID)
		if signingFingerprint != "" && v.Fingerprint == signingFingerprint && ev.SigningKeyVersionID == nil {
			id := v.ID
			ev.SigningKeyVersionID = &id
		}
	}
	for _, v := range s.Decrypt {
		ev.CandidateKeyVersionIDs = append(ev.CandidateKeyVersionIDs, v.ID)
		if decryptionFingerprint != "" && v.Fingerprint == decryptionFingerprint && ev.DecryptionKeyVersionID == nil {
			id := v.ID
			ev.DecryptionKeyVersionID = &id
		}
	}
	return ev
}

// EscrowRunKeyEvidence records which parties, arrangement revisions and key
// versions a validation run resolved and used. It complements, never replaces,
// the fingerprints the run already records: fingerprints say which key, these
// say which registry decision put that key in reach.
type EscrowRunKeyEvidence struct {
	DepositorPartyID            *uuid.UUID  `json:"depositorPartyId,omitempty"`
	ReceiverPartyID             *uuid.UUID  `json:"receiverPartyId,omitempty"`
	TLDArrangementRevision      int         `json:"tldArrangementRevision,omitempty"`
	OperatorArrangementRevision int         `json:"operatorArrangementRevision,omitempty"`
	PlatformArrangementRevision int         `json:"platformArrangementRevision,omitempty"`
	SigningKeyVersionID         *uuid.UUID  `json:"signingKeyVersionId,omitempty"`
	DecryptionKeyVersionID      *uuid.UUID  `json:"decryptionKeyVersionId,omitempty"`
	CandidateKeyVersionIDs      []uuid.UUID `json:"candidateKeyVersionIds,omitempty"`
}

// IsZero reports whether nothing was recorded: an unsigned profile, or a run
// not yet finalised.
func (e EscrowRunKeyEvidence) IsZero() bool {
	return e.DepositorPartyID == nil && e.ReceiverPartyID == nil &&
		e.SigningKeyVersionID == nil && e.DecryptionKeyVersionID == nil && len(e.CandidateKeyVersionIDs) == 0
}
