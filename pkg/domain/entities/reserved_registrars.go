package entities

// SpecialReservedGurIDs are the IANA reserved GurIDs that should be
// created as system registrars despite having "Reserved" IANA status.
// These are global (per-system) reservations that exist in every ICANN
// registry's registrar table.
//
// See: https://www.iana.org/assignments/registrar-ids/registrar-ids.xhtml
var SpecialReservedGurIDs = map[int]string{
	9995: "Reserved for Pre-Delegation Testing transactions #1 reporting",
	9996: "Reserved for Pre-Delegation Testing transactions #2 reporting",
	9997: "Reserved for ICANN's Registry SLA Monitoring System transactions reporting",
}

// SpecialReservedClIDs provides explicit, descriptive ClIDs for the
// special reserved registrars. Without these, CreateClID() truncates
// their long names to indistinguishable slugs like "999x-reserved-fo".
var SpecialReservedClIDs = map[int]string{
	9995: "9995-pdt-1",
	9996: "9996-pdt-2",
	9997: "9997-sla-monitor",
}

// TLDScopedReservedGurIDs are the IANA reserved GurIDs that need a
// per-TLD registrar account (e.g. 9998-com, 9999-com). These represent
// transactions where the Registry Operator acts as Registrar.
var TLDScopedReservedGurIDs = map[int]string{
	9998: "Reserved for billable transactions where Registry Operator acts as Registrar",
	9999: "Reserved for non-billable transactions where Registry Operator acts as Registrar",
}

// IsSpecialReservedGurID returns true if the given GurID is a global
// reserved registrar that should bypass the "skip reserved" filter
// during IANA sync. These registrars are created with platform status
// forced to OK so they can transact on the system.
func IsSpecialReservedGurID(gurID int) bool {
	_, ok := SpecialReservedGurIDs[gurID]
	return ok
}

// SpecialReservedClID returns the explicit ClID for a special reserved
// GurID, and true if one exists. Returns "", false for non-special IDs.
func SpecialReservedClID(gurID int) (string, bool) {
	clid, ok := SpecialReservedClIDs[gurID]
	return clid, ok
}

