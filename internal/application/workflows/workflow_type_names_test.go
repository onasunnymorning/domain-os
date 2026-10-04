package workflows

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// TestWorkflowTypeNames pins the registered Temporal type name of every
// workflow whose function ends in "Workflow" to that function name minus the
// suffix. The type name is persisted in history and in schedule actions, so it
// must not drift from the convention by accident.
func TestWorkflowTypeNames(t *testing.T) {
	cases := []struct {
		fn       interface{}
		typeName string
	}{
		{CheckSerialDriftWorkflow, CheckSerialDriftTypeName},
		{EscrowImportWorkflow, EscrowImportTypeName},
		{EscrowIntakeWorkflow, EscrowIntakeTypeName},
		{EscrowIntakeSweepWorkflow, EscrowIntakeSweepTypeName},
		{EscrowKeyProbeWorkflow, EscrowKeyProbeTypeName},
		{EscrowSanitizeWorkflow, EscrowSanitizeTypeName},
		{EscrowValidationWorkflow, EscrowValidationTypeName},
		{RestoreWorkflow, RestoreTypeName},
		{Spec5SweepWorkflow, Spec5SweepTypeName},
		{SyncRegistrarsWorkflow, SyncRegistrarsTypeName},
		{SyncSpec5Workflow, SyncSpec5TypeName},
		{TLDCleanupWorkflow, TLDCleanupTypeName},
	}

	seen := map[string]bool{}
	for _, c := range cases {
		full := runtime.FuncForPC(reflect.ValueOf(c.fn).Pointer()).Name()
		fnName := full[strings.LastIndex(full, ".")+1:]

		if !strings.HasSuffix(fnName, "Workflow") {
			t.Errorf("%s: test case is for a function without the Workflow suffix", fnName)
			continue
		}
		if want := strings.TrimSuffix(fnName, "Workflow"); c.typeName != want {
			t.Errorf("%s registers as %q, want %q", fnName, c.typeName, want)
		}
		if strings.HasSuffix(c.typeName, "Workflow") {
			t.Errorf("%s: registered type %q must not end in Workflow", fnName, c.typeName)
		}
		if seen[c.typeName] {
			t.Errorf("registered type %q is used twice", c.typeName)
		}
		seen[c.typeName] = true
	}
}
