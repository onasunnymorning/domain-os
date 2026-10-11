package entities

import "testing"

func TestUniqueRegistrarName(t *testing.T) {
	t.Run("unused name is returned unchanged and recorded", func(t *testing.T) {
		used := map[string]struct{}{}
		if got := UniqueRegistrarName("Acme Registrar", used); got != "Acme Registrar" {
			t.Fatalf("got %q, want %q", got, "Acme Registrar")
		}
		if _, ok := used["Acme Registrar"]; !ok {
			t.Fatal("expected name to be recorded in used")
		}
	})

	t.Run("collisions get increasing suffixes", func(t *testing.T) {
		used := map[string]struct{}{}
		want := []string{"Acme", "Acme-2", "Acme-3"}
		for i, w := range want {
			if got := UniqueRegistrarName("Acme", used); got != w {
				t.Fatalf("call %d: got %q, want %q", i, got, w)
			}
		}
	})

	t.Run("collision is detected on the normalized form", func(t *testing.T) {
		used := map[string]struct{}{"Acme Inc": {}}
		if got := UniqueRegistrarName("Acme  Inc.", used); got != "Acme Inc-2" {
			t.Fatalf("got %q, want %q", got, "Acme Inc-2")
		}
	})

	t.Run("suffix that is already taken is skipped", func(t *testing.T) {
		used := map[string]struct{}{"Acme": {}, "Acme-2": {}}
		if got := UniqueRegistrarName("Acme", used); got != "Acme-3" {
			t.Fatalf("got %q, want %q", got, "Acme-3")
		}
	})
}
