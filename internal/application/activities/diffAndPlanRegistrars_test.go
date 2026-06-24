package activities

import (
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

		plan, err := DiffAndPlanRegistrars("corr", []entities.IANARegistrar{i}, []entities.RegistrarListItem{ex})
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

		plan, err := DiffAndPlanRegistrars("corr", []entities.IANARegistrar{i}, []entities.RegistrarListItem{ex})
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

		plan, err := DiffAndPlanRegistrars("corr", []entities.IANARegistrar{i}, []entities.RegistrarListItem{ex})
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

		plan, err := DiffAndPlanRegistrars("corr", []entities.IANARegistrar{i}, []entities.RegistrarListItem{ex})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Updates) != 0 {
			t.Fatalf("expected no updates when both statuses match, got %d", len(plan.Updates))
		}
	})

	t.Run("create when new accredited registrar not present", func(t *testing.T) {
		i := iana(1003, "New Registrar, Corp.", entities.IANARegistrarStatusAccredited)

		plan, err := DiffAndPlanRegistrars("corr", []entities.IANARegistrar{i}, nil)
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

		plan, err := DiffAndPlanRegistrars("corr", []entities.IANARegistrar{i}, nil)
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

		plan, err := DiffAndPlanRegistrars("corr", []entities.IANARegistrar{i}, nil)
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

		plan, err := DiffAndPlanRegistrars("corr", []entities.IANARegistrar{i}, []entities.RegistrarListItem{ex})
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
}
