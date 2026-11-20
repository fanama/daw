package session_test

import (
	"math"
	"testing"

	"gorecord/domain/session"
)

func TestNormalizeGainMovesPeakToTarget(t *testing.T) {
	gain, ok := session.NormalizeGainDB(0.25, session.NormalizeTargetDB)
	if !ok {
		t.Fatal("a non-silent peak must normalize")
	}
	want := session.NormalizeTargetDB - 20*math.Log10(0.25)
	if math.Abs(gain-want) > 1e-9 {
		t.Fatalf("expected %f dB, got %f", want, gain)
	}
}

func TestNormalizeGainRejectsSilence(t *testing.T) {
	if _, ok := session.NormalizeGainDB(0, session.NormalizeTargetDB); ok {
		t.Fatal("silence must not be normalized")
	}
}

func TestNormalizeGainStaysInsideTakeGainRange(t *testing.T) {
	gain, ok := session.NormalizeGainDB(0.000001, session.NormalizeTargetDB)
	if !ok {
		t.Fatal("a quiet peak must normalize")
	}
	if gain != 24 {
		t.Fatalf("expected the gain ceiling, got %f", gain)
	}
}
