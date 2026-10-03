// Package rdesynth generates synthetic RDE escrow deposits: a FULL deposit
// (RFC 8909 wrapper, RFC 9022 objects) for a TLD of the caller's choosing,
// populated with made-up registrars, domains, contacts, hosts and NNDNs, as a
// gzip-compressed XML stream.
//
// It exists so an operator can produce a realistic deposit of a chosen shape
// on demand — to load-test an import, exercise the escrow validator, or demo
// the escrow pages — without touching a real registry's data. Nothing in a
// generated deposit is real: names come from fixed word lists, e-mail
// addresses are under example.net, telephone numbers are in the +1.555 range
// and every address is in a documentation prefix (RFC 5737, RFC 3849).
//
// Two properties are load-bearing and tested:
//
//  1. A generated deposit is valid by this repository's own definition: the
//     rdevalidate XML validator reports nothing against it. Every contact and
//     host belongs to a domain, every reference a domain makes resolves, and
//     the header counts are exact.
//  2. Generation streams. The header counts follow from the parameters alone,
//     so the document is written in a single pass and memory stays flat
//     however many objects are asked for.
//
// Output is deterministic for a given Params, Seed and Watermark included.
//
// Like rdevalidate and rdesanitize it has no database, object-storage or
// Temporal dependency; the REST layer wires it to an HTTP response.
package rdesynth
