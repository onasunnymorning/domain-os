package entities

import "fmt"

// UniqueRegistrarName returns a registrar name that does not collide with any
// name already in used, and records the returned name in used.
//
// Registrar names (and nicknames, which default to the name) are unique in the
// registry, but IANA does not guarantee that its registrar names are. The first
// holder of a name keeps it; later holders get a "-2", "-3", ... suffix. Names
// are compared in their normalized form, the same form NewRegistrar stores.
// A name that does not collide is returned unchanged.
func UniqueRegistrarName(name string, used map[string]struct{}) string {
	base := NormalizeString(name)
	if _, taken := used[base]; !taken {
		used[base] = struct{}{}
		return name
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if _, taken := used[candidate]; !taken {
			used[candidate] = struct{}{}
			return candidate
		}
	}
}
