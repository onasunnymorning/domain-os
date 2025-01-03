package entities

import "testing"

func TestNewContactPolicy(t *testing.T) {
	cp := NewContactPolicy()

	if cp.RegistrantContactPolicy != ContactPolicyTypeRequired {
		t.Errorf("expected RegistrantPolicy to be %q, got %q", ContactPolicyTypeRequired, cp.RegistrantContactPolicy)
	}
	if cp.TechContactPolicy != ContactPolicyTypeRequired {
		t.Errorf("expected TechPolicy to be %q, got %q", ContactPolicyTypeRequired, cp.TechContactPolicy)
	}
	if cp.AdminContactPolicy != ContactPolicyTypeOptional {
		t.Errorf("expected AdminPolicy to be %q, got %q", ContactPolicyTypeOptional, cp.AdminContactPolicy)
	}
	if cp.BillingContactPolicy != ContactPolicyTypeOptional {
		t.Errorf("expected BillingPolicy to be %q, got %q", ContactPolicyTypeOptional, cp.BillingContactPolicy)
	}
}
