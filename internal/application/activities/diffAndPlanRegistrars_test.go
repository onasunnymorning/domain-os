package activities

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func TestDiffAndPlanRegistrars_BasicScenarios(t *testing.T) {
	// Helper to build IANA registrar
	iana := func(gurid int, name string, status entities.IANARegistrarStatus) entities.IANARegistrar {
		return entities.IANARegistrar{
			GurID:  gurid,
			Name:   name,
			Status: status,
		}
	}

	// Build existing registrar list item with same ClID as an IANA registrar
	existingFromIANA := func(ir entities.IANARegistrar, status entities.RegistrarStatus, ianaStatus entities.IANARegistrarStatus) entities.RegistrarListItem {
		clid, _ := ir.CreateClID()
		return entities.RegistrarListItem{
			ClID:       clid,
			Name:       ir.Name,
			GurID:      ir.GurID,
			Status:     status,
			IANAStatus: ianaStatus,
		}
	}

	t.Run("no update when accredited vs ok and IANA status matches", func(t *testing.T) {
		i := iana(1001, "Example Registrar, Inc.", entities.IANARegistrarStatusAccredited)
		ex := existingFromIANA(i, entities.RegistrarStatusOK, entities.IANARegistrarStatusAccredited)

		plan, err := DiffAndPlanRegistrars(context.Background(), "corr", []entities.IANARegistrar{i}, []entities.RegistrarListItem{ex})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Updates) != 0 {
			t.Fatalf("expected no updates, got %d", len(plan.Updates))
		}
		if len(plan.Creates) != 0 {
			t.Fatalf("expected no creates, got %d", len(plan.Creates))
		}
		if plan.SkippedReserved != 0 {
			t.Fatalf("expected skippedReserved=0, got %d", plan.SkippedReserved)
		}
	})

	t.Run("update when iana terminated and platform ok", func(t *testing.T) {
		i := iana(1002, "Terminated Registrar, LLC", entities.IANARegistrarStatusTerminated)
		ex := existingFromIANA(i, entities.RegistrarStatusOK, entities.IANARegistrarStatusAccredited)

		plan, err := DiffAndPlanRegistrars(context.Background(), "corr", []entities.IANARegistrar{i}, []entities.RegistrarListItem{ex})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Updates) != 1 {
			t.Fatalf("expected 1 update, got %d", len(plan.Updates))
		}
		if plan.Updates[0].NewStatus != string(entities.RegistrarStatusTerminated) {
			t.Fatalf("expected update to 'terminated', got %s", plan.Updates[0].NewStatus)
		}
		if plan.Updates[0].NewIANAStatus != string(entities.IANARegistrarStatusTerminated) {
			t.Fatalf("expected IANA status update to 'Terminated', got %s", plan.Updates[0].NewIANAStatus)
		}
	})

	t.Run("update only IANA status when platform status matches but IANA does not", func(t *testing.T) {
		i := iana(1003, "Mismatched IANA, Inc.", entities.IANARegistrarStatusAccredited)
		// Platform status is OK (correct) but IANA status is still Unknown
		ex := existingFromIANA(i, entities.RegistrarStatusOK, entities.IANARegistrarStatusUnknown)

		plan, err := DiffAndPlanRegistrars(context.Background(), "corr", []entities.IANARegistrar{i}, []entities.RegistrarListItem{ex})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Updates) != 1 {
			t.Fatalf("expected 1 update for IANA status drift, got %d", len(plan.Updates))
		}
		// Platform status should NOT change
		if plan.Updates[0].NewStatus != "" {
			t.Fatalf("expected no platform status change, got %q", plan.Updates[0].NewStatus)
		}
		// IANA status should update
		if plan.Updates[0].NewIANAStatus != string(entities.IANARegistrarStatusAccredited) {
			t.Fatalf("expected IANA status update to 'Accredited', got %s", plan.Updates[0].NewIANAStatus)
		}
	})

	t.Run("no update when both statuses already match", func(t *testing.T) {
		i := iana(1004, "All Good Registrar", entities.IANARegistrarStatusTerminated)
		ex := existingFromIANA(i, entities.RegistrarStatusTerminated, entities.IANARegistrarStatusTerminated)

		plan, err := DiffAndPlanRegistrars(context.Background(), "corr", []entities.IANARegistrar{i}, []entities.RegistrarListItem{ex})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Updates) != 0 {
			t.Fatalf("expected no updates when both statuses match, got %d", len(plan.Updates))
		}
	})

	t.Run("create when new accredited registrar not present", func(t *testing.T) {
		i := iana(1003, "New Registrar, Corp.", entities.IANARegistrarStatusAccredited)

		plan, err := DiffAndPlanRegistrars(context.Background(), "corr", []entities.IANARegistrar{i}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Creates) != 1 {
			t.Fatalf("expected 1 create, got %d", len(plan.Creates))
		}
		if plan.SkippedReserved != 0 {
			t.Fatalf("expected skippedReserved=0, got %d", plan.SkippedReserved)
		}
	})

	t.Run("skip reserved registrar when not special GurIDs", func(t *testing.T) {
		i := iana(2001, "Reserved Registrar", entities.IANARegistrarStatusReserved)

		plan, err := DiffAndPlanRegistrars(context.Background(), "corr", []entities.IANARegistrar{i}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Creates) != 0 {
			t.Fatalf("expected 0 creates, got %d", len(plan.Creates))
		}
		if plan.SkippedReserved != 1 {
			t.Fatalf("expected skippedReserved=1, got %d", plan.SkippedReserved)
		}
	})

	t.Run("create reserved for special GurID 9995", func(t *testing.T) {
		i := iana(9995, "Pre-Delegation Testing Registrar", entities.IANARegistrarStatusReserved)

		plan, err := DiffAndPlanRegistrars(context.Background(), "corr", []entities.IANARegistrar{i}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Creates) != 1 {
			t.Fatalf("expected 1 create for 9995, got %d", len(plan.Creates))
		}
		if plan.SkippedReserved != 0 {
			t.Fatalf("expected skippedReserved=0 for special ID, got %d", plan.SkippedReserved)
		}
	})

	t.Run("special GurID 9995 forces OK when existing", func(t *testing.T) {
		i := iana(9995, "Pre-Delegation Testing", entities.IANARegistrarStatusReserved)
		ex := existingFromIANA(i, entities.RegistrarStatusReadonly, entities.IANARegistrarStatusReserved)

		plan, err := DiffAndPlanRegistrars(context.Background(), "corr", []entities.IANARegistrar{i}, []entities.RegistrarListItem{ex})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Updates) != 1 {
			t.Fatalf("expected 1 update for special GurID 9995, got %d", len(plan.Updates))
		}
		if plan.Updates[0].NewStatus != string(entities.RegistrarStatusOK) {
			t.Fatalf("expected forced OK for 9995, got %q", plan.Updates[0].NewStatus)
		}
	})

	t.Run("no update for special GurID 9995 already in sync", func(t *testing.T) {
		// Special reserved registrar already at platform status ok with IANA
		// status Reserved: it is fully in sync and must NOT produce an update
		// on every run (the recurring "status set to ok" noise).
		i := iana(9995, "Pre-Delegation Testing", entities.IANARegistrarStatusReserved)
		ex := existingFromIANA(i, entities.RegistrarStatusOK, entities.IANARegistrarStatusReserved)

		plan, err := DiffAndPlanRegistrars(context.Background(), "corr", []entities.IANARegistrar{i}, []entities.RegistrarListItem{ex})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Updates) != 0 {
			t.Fatalf("expected 0 updates for in-sync special GurID 9995, got %d", len(plan.Updates))
		}
		if len(plan.Creates) != 0 {
			t.Fatalf("expected 0 creates for existing special GurID 9995, got %d", len(plan.Creates))
		}
	})

	t.Run("create reserved for special GurID 9997", func(t *testing.T) {
		i := iana(9997, "ICANN SLA Monitoring", entities.IANARegistrarStatusReserved)

		plan, err := DiffAndPlanRegistrars(context.Background(), "corr", []entities.IANARegistrar{i}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Creates) != 1 {
			t.Fatalf("expected 1 create for 9997, got %d", len(plan.Creates))
		}
		if plan.SkippedReserved != 0 {
			t.Fatalf("expected skippedReserved=0 for special ID, got %d", plan.SkippedReserved)
		}
	})

	t.Run("special GurID 9997 forces OK when existing", func(t *testing.T) {
		i := iana(9997, "ICANN SLA Monitoring", entities.IANARegistrarStatusReserved)
		ex := existingFromIANA(i, entities.RegistrarStatusReadonly, entities.IANARegistrarStatusReserved)

		plan, err := DiffAndPlanRegistrars(context.Background(), "corr", []entities.IANARegistrar{i}, []entities.RegistrarListItem{ex})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Updates) != 1 {
			t.Fatalf("expected 1 update for special GurID 9997, got %d", len(plan.Updates))
		}
		if plan.Updates[0].NewStatus != string(entities.RegistrarStatusOK) {
			t.Fatalf("expected forced OK for 9997, got %q", plan.Updates[0].NewStatus)
		}
	})
}

// Registrar names are unique in the registry but IANA's are not. A create whose
// name is already taken used to be planned as-is, silently dropped by the insert
// (ON CONFLICT DO NOTHING) and then re-planned on every daily run.
func TestDiffAndPlanRegistrars_NameCollisions(t *testing.T) {
	iana := func(gurID int, name string) entities.IANARegistrar {
		return entities.IANARegistrar{GurID: gurID, Name: name, Status: entities.IANARegistrarStatusAccredited}
	}
	existing := func(ir entities.IANARegistrar) entities.RegistrarListItem {
		clid, _ := ir.CreateClID()
		return entities.RegistrarListItem{
			ClID:       clid,
			Name:       ir.Name,
			GurID:      ir.GurID,
			Status:     entities.RegistrarStatusOK,
			IANAStatus: entities.IANARegistrarStatusAccredited,
		}
	}
	plan := func(t *testing.T, ianas []entities.IANARegistrar, ex []entities.RegistrarListItem) DiffPlanResult {
		t.Helper()
		p, err := DiffAndPlanRegistrars(context.Background(), "corr", ianas, ex)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return p
	}

	t.Run("name taken by an existing registrar gets a suffix", func(t *testing.T) {
		held := iana(100, "Shared Name Inc")
		newcomer := iana(900, "Shared Name Inc")

		p := plan(t, []entities.IANARegistrar{held, newcomer}, []entities.RegistrarListItem{existing(held)})

		if len(p.Creates) != 1 {
			t.Fatalf("expected 1 create, got %d", len(p.Creates))
		}
		if p.Creates[0].Name != "Shared Name Inc-2" {
			t.Fatalf("expected suffixed name, got %q", p.Creates[0].Name)
		}
		if len(p.RenamedForUniqueness) != 1 || p.RenamedForUniqueness[0] != p.Creates[0].ClID {
			t.Fatalf("expected the create to be reported as renamed, got %v", p.RenamedForUniqueness)
		}
	})

	t.Run("two new registrars sharing a name in one plan", func(t *testing.T) {
		a := iana(901, "Twin Registrar")
		b := iana(902, "Twin Registrar")

		p := plan(t, []entities.IANARegistrar{a, b}, nil)

		if len(p.Creates) != 2 {
			t.Fatalf("expected 2 creates, got %d", len(p.Creates))
		}
		if p.Creates[0].Name != "Twin Registrar" || p.Creates[1].Name != "Twin Registrar-2" {
			t.Fatalf("unexpected names: %q, %q", p.Creates[0].Name, p.Creates[1].Name)
		}
	})

	t.Run("a collision on the normalized form is caught", func(t *testing.T) {
		held := iana(100, "Dotted Name Ltd")
		newcomer := iana(903, "Dotted Name Ltd.")

		p := plan(t, []entities.IANARegistrar{held, newcomer}, []entities.RegistrarListItem{existing(held)})

		if len(p.Creates) != 1 || p.Creates[0].Name != "Dotted Name Ltd-2" {
			t.Fatalf("expected one create named %q, got %+v", "Dotted Name Ltd-2", p.Creates)
		}
	})

	t.Run("converges once the suffixed registrar exists", func(t *testing.T) {
		held := iana(100, "Shared Name Inc")
		newcomer := iana(900, "Shared Name Inc")
		created := existing(newcomer)
		created.Name = "Shared Name Inc-2"

		p := plan(t, []entities.IANARegistrar{held, newcomer}, []entities.RegistrarListItem{existing(held), created})

		if len(p.Creates) != 0 {
			t.Fatalf("expected no creates on the second run, got %d", len(p.Creates))
		}
		if len(p.RenamedForUniqueness) != 0 {
			t.Fatalf("expected nothing renamed, got %v", p.RenamedForUniqueness)
		}
	})

	t.Run("non-colliding create keeps its name", func(t *testing.T) {
		p := plan(t, []entities.IANARegistrar{iana(904, "Unique Registrar")}, nil)

		if len(p.Creates) != 1 || p.Creates[0].Name != "Unique Registrar" {
			t.Fatalf("expected an unchanged name, got %+v", p.Creates)
		}
		if len(p.RenamedForUniqueness) != 0 {
			t.Fatalf("expected nothing renamed, got %v", p.RenamedForUniqueness)
		}
	})
}
