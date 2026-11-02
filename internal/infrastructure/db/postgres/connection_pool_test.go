package postgres

import "testing"

// TestPoolSize covers Config.MaxOpenConns: the default for zero, the value
// when it is at least the minimum, and an error below it (one connection
// hangs a migration, and a negative one database/sql reads as unbounded).
func TestPoolSize(t *testing.T) {
	cases := []struct {
		name string
		n    int
		want int
		err  bool
	}{
		{"zero is the default", 0, DefaultMaxOpenConns, false},
		{"the minimum", 2, 2, false},
		{"larger", 7, 7, false},
		{"one hangs a migration", 1, 0, true},
		{"negative is unbounded", -3, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := poolSize(tc.n)
			if (err != nil) != tc.err {
				t.Fatalf("poolSize(%d): err = %v, want error %v", tc.n, err, tc.err)
			}
			if got != tc.want {
				t.Errorf("poolSize(%d) = %d, want %d", tc.n, got, tc.want)
			}
		})
	}
}
