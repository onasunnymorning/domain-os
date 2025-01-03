package entities

import "testing"

func TestNewContactPolicy(t *testing.T) {
	cp := NewContactPolicy()

	if cp.RegistrantContactPolicy != ContactDataPolicyTypeRequired {
		t.Errorf("expected RegistrantPolicy to be %q, got %q", ContactDataPolicyTypeRequired, cp.RegistrantContactPolicy)
	}
	if cp.TechContactPolicy != ContactDataPolicyTypeRequired {
		t.Errorf("expected TechPolicy to be %q, got %q", ContactDataPolicyTypeRequired, cp.TechContactPolicy)
	}
	if cp.AdminContactPolicy != ContactDataPolicyTypeOptional {
		t.Errorf("expected AdminPolicy to be %q, got %q", ContactDataPolicyTypeOptional, cp.AdminContactPolicy)
	}
	if cp.BillingContactPolicy != ContactDataPolicyTypeOptional {
		t.Errorf("expected BillingPolicy to be %q, got %q", ContactDataPolicyTypeOptional, cp.BillingContactPolicy)
	}
}
