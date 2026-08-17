package validator

// PB-05 DATA VALIDATOR - Satpam Terakhir
// Hanya boleh PASS atau DROP, tidak boleh ubah byte

import (
	"github.com/otris-id/otris-pemecah-batu/internal/model"
)

type Validator struct {
	metrics model.ValidatorMetrics
}

func New() *Validator {
	return &Validator{}
}

func (v *Validator) Process(in model.TimestampedChunk) model.ValidatedChunk {
	out := model.ValidatedChunk{
		TimestampedChunk: in,
		Valid: true,
	}

	v.metrics.TotalBytes += int64(len(in.Bytes))

	// Rule 1: panjang 0 atau >64KB = malformed
	if len(in.Bytes) == 0 {
		out.Valid = false
		out.DropReason = "panjang 0"
		v.metrics.DroppedMalformed++
		return out
	}
	if len(in.Bytes) > 64*1024 {
		out.Valid = false
		out.DropReason = "panjang >64KB"
		v.metrics.DroppedMalformed++
		return out
	}

	// Rule 2: suspect dari PB-02 tetap PASS, tapi hitung
	if in.Suspect {
		v.metrics.SuspectFlagged++
		// tetap PASS, jangan DROP, karena bisa jadi data beneran
	}

	v.metrics.ValidFrames++
	return out
}

func (v *Validator) Metrics() model.ValidatorMetrics {
	return v.metrics
}
