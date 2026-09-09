// Package rdevalidate is the pure escrow-verification (EVE) core for issue
// #412: it verifies a detached OpenPGP signature over a .ryde deposit,
// decrypts it with the service keyring, unpacks the payload under explicit
// limits, streams the RDE XML through strict structural and content checks,
// and turns everything it learns into a structured, auditable Result.
//
// It has no database, object-storage or Temporal dependency, in the same way
// internal/application/serialdrift holds pure decision logic: the activities
// layer wires it to the outside world. Nothing in here may call, or import,
// the escrow import pipeline (design constraint 1: validation is not import).
//
// Every failure is a Finding with a stable Code. Messages and locators carry
// constant text plus numbers only — never untrusted file names, object names
// or payload content (design constraint 6).
package rdevalidate
