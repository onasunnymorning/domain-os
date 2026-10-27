package postgres

import "testing"

// TestMaxOpenConnsFromEnv covers DB_MAX_OPEN_CONNS: the default when unset,
// the value when valid, and the default again (never zero or negative, which
// database/sql reads as unbounded) when it is not a positive integer.
func TestMaxOpenConnsFromEnv(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want int
	}{
		{"unset", "", defaultMaxOpenConns},
		{"valid", "7", 7},
		{"zero", "0", defaultMaxOpenConns},
		{"negative", "-3", defaultMaxOpenConns},
		{"not a number", "many", defaultMaxOpenConns},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(maxOpenConnsEnv, tc.env)
			if got := maxOpenConnsFromEnv(); got != tc.want {
				t.Errorf("DB_MAX_OPEN_CONNS=%q: got %d, want %d", tc.env, got, tc.want)
			}
		})
	}
}
