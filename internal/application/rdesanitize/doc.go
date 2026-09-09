// Package rdesanitize turns a validated RDE escrow deposit into a
// sanitized-pseudonymized derivative for internal analytics and non-production
// testing (issue #415).
//
// The derivative is an adjunct to escrow custody, never a replacement for it.
// Escrow data exists to support recovery, and ICANN's RDE material requires the
// archived deposited copy to stay unmodified, so nothing here writes to, moves
// or re-labels the source object. A derivative is a separate processing
// purpose with its own access and retention rules, and it carries the label
// entities.EscrowDerivativeLabel — "sanitized-pseudonymized", never "anonymous".
//
// Three constraints shape the implementation:
//
//  1. It is a token-level rewriter, not a struct round-trip. The RDE entity
//     structs in pkg/domain/entities model neither authInfo nor
//     secDNS:keyData and carry no namespace prefixes, so decoding into them and
//     re-marshalling would silently drop published DNSSEC key material and emit
//     a fabricated authInfo. Rewriting tokens preserves everything the profile
//     does not deliberately change.
//
//  2. It fails closed. An element, attribute or namespace the versioned
//     profile does not classify quarantines the run; nothing is written. An
//     unclassified field is not a field we may assume is safe.
//
//  3. No raw field value ever leaves this package. Findings, logs, manifests,
//     counters and object names carry constant templates, numbers and reason
//     codes only. Locators identify a position (element index, byte offset),
//     never content.
//
// Like rdevalidate, this package is pure: no database, no object storage, no
// Temporal, and it never calls the escrow import pipeline.
package rdesanitize
