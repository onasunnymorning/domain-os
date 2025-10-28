package activities

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
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
	existingFromIANA := func(ir entities.IANARegistrar, status entities.RegistrarStatus) entities.RegistrarListItem {
		clid, _ := ir.CreateClID()
		return entities.RegistrarListItem{
			ClID:   clid,
			Name:   ir.Name,
			GurID:  ir.GurID,
			Status: status,
		}
	}

	t.Run("no update when accredited vs ok", func(t *testing.T) {
		i := iana(1001, "Example Registrar, Inc.", entities.IANARegistrarStatusAccredited)
		ex := existingFromIANA(i, entities.RegistrarStatusOK)

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
		ex := existingFromIANA(i, entities.RegistrarStatusOK)

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
}
