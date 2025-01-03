package entities

import "testing"

func TestNewContactPolicy(t *testing.T) {
	cp := NewContactPolicy()

	if cp.RegistrantContactDataPolicy != ContactDataPolicyTypeRequired {
		t.Errorf("expected RegistrantPolicy to be %q, got %q", ContactDataPolicyTypeRequired, cp.RegistrantContactDataPolicy)
	}
	if cp.TechContactDataPolicy != ContactDataPolicyTypeRequired {
		t.Errorf("expected TechPolicy to be %q, got %q", ContactDataPolicyTypeRequired, cp.TechContactDataPolicy)
	}
	if cp.AdminContactDataPolicy != ContactDataPolicyTypeOptional {
		t.Errorf("expected AdminPolicy to be %q, got %q", ContactDataPolicyTypeOptional, cp.AdminContactDataPolicy)
	}
	if cp.BillingContactDataPolicy != ContactDataPolicyTypeOptional {
		t.Errorf("expected BillingPolicy to be %q, got %q", ContactDataPolicyTypeOptional, cp.BillingContactDataPolicy)
	}
}
