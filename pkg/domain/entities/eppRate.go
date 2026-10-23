package entities

import "time"

// EPPRateBucket is one token bucket a command takes from: its registrar's
// or its family's, for the command's class (docs/epp/ARCHITECTURE.md §5).
// Key names the bucket across every node.
type EPPRateBucket struct {
	Key  string
	Rate EPPRate
}

// EPPRateTake is the answer to taking tokens from a set of buckets at once.
type EPPRateTake struct {
	// Taken is whether the tokens were taken. They are taken when every
	// bucket can give them within the longest wait the caller accepts.
	Taken bool
	// Wait is how long the command must wait for the tokens: before it
	// runs when Taken, before it could be tried again when not.
	Wait time.Duration
	// Limiting is the index of the bucket with the longest wait, when Wait
	// is above zero.
	Limiting int
}
