package session_test

import (
	"math"
	"testing"

	"fanamix/domain/session"
)

func TestSetPanClampsToStereoField(t *testing.T) {
	current := newSession(t)
	track := addTrack(t, current, "Guitar", 0)
	if track.Pan() != 0 {
		t.Fatalf("a new track must be centred, got %v", track.Pan())
	}
	if err := current.SetPan(track.ID(), -3.5); err != nil {
		t.Fatalf("set pan: %v", err)
	}
	reloaded, ok := current.FindTrack(track.ID())
	if !ok || reloaded.Pan() != session.MinTrackPan {
		t.Fatalf("expected hard left, got %v", reloaded.Pan())
	}
	if err := current.SetPan(track.ID(), 0.35); err != nil {
		t.Fatalf("set pan: %v", err)
	}
	if reloaded, _ := current.FindTrack(track.ID()); math.Abs(reloaded.Pan()-0.35) > 1e-9 {
		t.Fatalf("expected pan 0.35, got %v", reloaded.Pan())
	}
}

func TestSetPanRejectsNonFiniteValues(t *testing.T) {
	current := newSession(t)
	track := addTrack(t, current, "Guitar", 0)
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := current.SetPan(track.ID(), value); !session.IsCode(err, session.CodeInvalid) {
			t.Fatalf("expected invalid pan for %v, got %v", value, err)
		}
	}
}
