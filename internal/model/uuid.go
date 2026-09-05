package model

import (
	"crypto/rand"
	"encoding/hex"
)

// NewID mints one random UUID (version 4, variant 1) in the canonical
// hyphenated text form.
//
// SPEC.md 3.3 types run_id, step_id, attempt_id and orchestrator_id as UUIDs
// and says nothing about how they are produced, so the only requirement is
// that they be unique. crypto/rand is used rather than the database's
// gen_random_uuid() because SPEC.md 3.3 makes orchestrator_id "generated fresh
// at each process boot", which happens before the first query, and because an
// identity minted in the application is available to the caller without a
// round trip.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand.Read never returns an error on any platform Go
		// supports; if it somehow did, continuing would mint colliding
		// identities, which is worse than stopping.
		panic("piton: cannot read random bytes for a UUID: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 1

	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], b[10:16])
	return string(out[:])
}
