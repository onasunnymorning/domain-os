package rdesanitize

import (
	"encoding/json"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Manifest travels with a derivative and is the only description of it anyone
// downstream reads. It carries counts, identifiers and reason codes — never a
// value, an excerpt, a file name from the source, or anything derived from
// deposit content.
type Manifest struct {
	// Label is always entities.EscrowDerivativeLabel. It is written out so a
	// consumer that reads only the manifest still learns that this data is
	// pseudonymized rather than anonymous.
	Label string `json:"label"`

	TenantID string `json:"tenantId"`
	TLD      string `json:"tld"`
	// SyntheticSuffix is the suffix every in-bailiwick name was rewritten to.
	SyntheticSuffix string `json:"syntheticSuffix"`

	SourceValidationRunID string `json:"sourceValidationRunId"`
	SourceDepositID       string `json:"sourceDepositId"`
	SourceProfile         string `json:"sourceProfile"`
	// SourceArtifactSHA256 is the immutable version of the object this was
	// derived from. The source itself is never modified.
	SourceArtifactSHA256 string `json:"sourceArtifactSha256"`

	DerivativeObjectKey string `json:"derivativeObjectKey"`
	DerivativeSHA256    string `json:"derivativeSha256"`
	DerivativeBytes     int64  `json:"derivativeBytes"`

	PolicyVersion   string `json:"policyVersion"`
	WorkflowVersion string `json:"workflowVersion"`
	// TokenKeyFingerprint identifies the HMAC key without revealing it, so a
	// rotation shows up as a visible break in joinability instead of a silent
	// one. It is a MAC over a fixed public string and cannot be inverted.
	TokenKeyFingerprint string `json:"tokenKeyFingerprint"`

	WorkflowID string `json:"workflowId"` // correlation id (INV-16)
	RunID      string `json:"runId"`      // trace id

	Outcome     string    `json:"outcome"`
	ReasonCodes []string  `json:"reasonCodes,omitempty"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`

	SourceElements int64                             `json:"sourceElements"`
	Counts         entities.EscrowSanitizationCounts `json:"counts"`
}

// NewManifest assembles the manifest for a finished run.
func NewManifest(run *entities.EscrowSanitizationRun, res Result, sourceProfile, tokenKeyFingerprint, derivativeObjectKey, derivativeSHA256 string, derivativeBytes int64) Manifest {
	codes := make([]string, 0, len(res.Findings))
	for _, c := range res.Codes() {
		codes = append(codes, string(c))
	}
	return Manifest{
		Label:                 entities.EscrowDerivativeLabel,
		TenantID:              run.TenantID.String(),
		TLD:                   run.TLD,
		SyntheticSuffix:       run.SyntheticSuffix,
		SourceValidationRunID: run.SourceValidationRunID.String(),
		SourceDepositID:       run.SourceDepositID.String(),
		SourceProfile:         sourceProfile,
		SourceArtifactSHA256:  run.SourceArtifactSHA256,
		DerivativeObjectKey:   derivativeObjectKey,
		DerivativeSHA256:      derivativeSHA256,
		DerivativeBytes:       derivativeBytes,
		PolicyVersion:         run.PolicyVersion,
		WorkflowVersion:       run.WorkflowVersion,
		TokenKeyFingerprint:   tokenKeyFingerprint,
		WorkflowID:            run.WorkflowID,
		RunID:                 run.RunID,
		Outcome:               string(res.Outcome),
		ReasonCodes:           codes,
		StartedAt:             res.StartedAt,
		CompletedAt:           res.CompletedAt,
		SourceElements:        res.SourceElements,
		Counts:                res.Counts,
	}
}

// Marshal renders the manifest as indented JSON.
func (m Manifest) Marshal() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}
