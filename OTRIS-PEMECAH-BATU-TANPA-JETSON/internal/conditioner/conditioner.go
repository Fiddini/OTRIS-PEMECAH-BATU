package conditioner

// PB-02 SIGNAL CONDITIONER - Tukang Saring
// Hukum: kalau ragu apakah noise atau data, JANGAN HAPUS

import (
	"github.com/otris-id/otris-pemecah-batu/internal/model"
)

type Conditioner struct {
	zeroStreak int
}

func New() *Conditioner {
	return &Conditioner{}
}

func (c *Conditioner) Process(in model.RawChunk) model.ConditionedChunk {
	out := model.ConditionedChunk{
		RawChunk: in,
		Suspect: false,
	}

	// Deteksi kabel kendor: 10x 0x00 berturut
	zeroCount := 0
	for _, b := range in.Bytes {
		if b == 0x00 {
			zeroCount++
			if zeroCount >= 10 {
				out.Suspect = true
				out.SuspectReason = "10x 0x00 berturut - kemungkinan kabel kendor"
				break
			}
		} else {
			zeroCount = 0
		}
	}

	// Jitter sederhana: kalau bytes < 5, suspect
	if len(in.Bytes) < 5 && !out.Suspect {
		out.Suspect = true
		out.SuspectReason = "frame terlalu pendek <5 bytes"
	}

	return out
}
