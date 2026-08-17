package model

import "time"

// RawChunk dari PB-01, kebenaran mutlak
type RawChunk struct {
	Bytes      []byte
	ReceivedAt time.Time // monotonic + wall
	Source     string    // tcp://192.168.1.50:4000
}

// ConditionedChunk dari PB-02
type ConditionedChunk struct {
	RawChunk
	Suspect       bool
	SuspectReason string
	JitterMs      float64
}

// TimestampedChunk dari PB-03
type TimestampedChunk struct {
	ConditionedChunk
	WallUTC     time.Time
	MonotonicNS int64
	NTPSynced   bool
	DriftMs     float64
}

// ValidatedChunk dari PB-05
type ValidatedChunk struct {
	TimestampedChunk
	Valid bool
	DropReason string
}

// Metrics dari PB-05
type ValidatorMetrics struct {
	TotalBytes       int64
	ValidFrames      int64
	DroppedCRC       int64
	DroppedMalformed int64
	SuspectFlagged   int64
}
