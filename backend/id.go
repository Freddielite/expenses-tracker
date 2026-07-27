package main

import (
	"crypto/rand"
	"encoding/hex"
)

// newID generates a short random hex identifier using only the standard
// library. Good enough for a single-user local app; no need to pull in an
// external UUID package for this.
func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Extremely unlikely; crypto/rand.Read only fails on a broken system.
		panic(err)
	}
	return hex.EncodeToString(b)
}
