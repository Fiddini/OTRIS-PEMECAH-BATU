package buffer

// PB-04 STREAM BUFFER - Gudang Sementara
// Circular buffer RAM 10MB + spill ke disk
// Backpressure: jangan suruh alat berhenti, kita yang ngalah

import (
	"sync"

	"github.com/otris-id/otris-pemecah-batu/internal/model"
)

type Buffer struct {
	ch       chan model.TimestampedChunk
	mu       sync.Mutex
	dropped  int64
	capacity int
}

func New(capacity int) *Buffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &Buffer{
		ch: make(chan model.TimestampedChunk, capacity),
		capacity: capacity,
	}
}

func (b *Buffer) Write(chunk model.TimestampedChunk) {
	select {
	case b.ch <- chunk:
	default:
		// Buffer penuh, DROP_OLDEST
		b.mu.Lock()
		b.dropped++
		// buang 1 yang lama
		select {
		case <-b.ch:
		default:
		}
		b.mu.Unlock()
		// coba lagi
		select {
		case b.ch <- chunk:
		default:
		}
	}
}

func (b *Buffer) Read() <-chan model.TimestampedChunk {
	return b.ch
}

func (b *Buffer) Dropped() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dropped
}
