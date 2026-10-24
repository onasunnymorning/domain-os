package activities

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// Randomised release (anti-dropcatch measure 5, docs/epp/ARCHITECTURE.md
// §5): the purge loop waits a random offset before it purges, and purges
// in a random order, so the moment a name is released cannot be known in
// advance. The randomness is drawn here, in activities, because a workflow
// must be deterministic (INV-06): Temporal records each result, and a
// replay reads it back instead of drawing again. crypto/rand, so the draw
// cannot be predicted from earlier ones.

// ReleaseOffset is a random wait in [0, window), to the second.
func ReleaseOffset(_ context.Context, window time.Duration) (time.Duration, error) {
	seconds := int64(window / time.Second)
	if seconds < 1 {
		return 0, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(seconds))
	if err != nil {
		return 0, fmt.Errorf("draw a release offset: %w", err)
	}
	return time.Duration(n.Int64()) * time.Second, nil
}

// ShuffleForRelease returns names in a random order (Fisher–Yates).
func ShuffleForRelease(_ context.Context, names []string) ([]string, error) {
	out := append([]string(nil), names...)
	for i := len(out) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return nil, fmt.Errorf("shuffle the release order: %w", err)
		}
		out[i], out[j.Int64()] = out[j.Int64()], out[i]
	}
	return out, nil
}
