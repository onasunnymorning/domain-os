package entities

import "testing"

func TestNewContactPolicy(t *testing.T) {
	cp := NewContactPolicy()

	if cp.RegistrantContactDataPolicy != ContactDataPolicyTypeMandatory {
		t.Errorf("expected RegistrantPolicy to be %q, got %q", ContactDataPolicyTypeMandatory, cp.RegistrantContactDataPolicy)
	}
	if cp.TechContactDataPolicy != ContactDataPolicyTypeMandatory {
		t.Errorf("expected TechPolicy to be %q, got %q", ContactDataPolicyTypeMandatory, cp.TechContactDataPolicy)
	}
	if cp.AdminContactDataPolicy != ContactDataPolicyTypeOptional {
		t.Errorf("expected AdminPolicy to be %q, got %q", ContactDataPolicyTypeOptional, cp.AdminContactDataPolicy)
	}
	if cp.BillingContactDataPolicy != ContactDataPolicyTypeOptional {
		t.Errorf("expected BillingPolicy to be %q, got %q", ContactDataPolicyTypeOptional, cp.BillingContactDataPolicy)
	}
}
