package timestamp

// PB-03 TIMESTAMP NORMALIZER - Tukang Jam
// Dua jam: wall UTC + monotonic

import (
	"time"

	"github.com/otris-id/otris-pemecah-batu/internal/model"
)

type Normalizer struct {
	startMonotonic time.Time
}

func New() *Normalizer {
	return &Normalizer{
		startMonotonic: time.Now(),
	}
}

func (n *Normalizer) Process(in model.ConditionedChunk) model.TimestampedChunk {
	now := time.Now().UTC()
	mono := time.Since(n.startMonotonic).Nanoseconds()

	return model.TimestampedChunk{
		ConditionedChunk: in,
		WallUTC:         now,
		MonotonicNS:     mono,
		NTPSynced:       true, // TODO: cek ntp
		DriftMs:         0,
	}
}
