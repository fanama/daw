package session_test

import (
	"testing"
	"time"

	"fanamix/domain/session"
)

func TestSplitTakeSharesTheRecordingBetweenHalves(t *testing.T) {
	s := newSession(t)
	guitar := addTrack(t, s, "Guitar", 0)
	take := recordTake(t, s, guitar.ID(), "takes/take-001.wav", 0)

	second, err := s.SplitTake(take.ID(), 10000, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	first, ok := s.FindTake(take.ID())
	if !ok {
		t.Fatal("first half disappeared")
	}
	if first.Trim().EndFrame != 10000 {
		t.Fatalf("expected the first half to end at the split, got %d", first.Trim().EndFrame)
	}
	if second.StartFrame() != 10000 || second.File() != take.File() {
		t.Fatalf("unexpected second half: %+v", second)
	}
	if second.Trim().StartFrame != 10000 || second.AudibleLength() != 14000 {
		t.Fatalf("unexpected second window: %+v", second.Trim())
	}
	if !second.IsLayered() {
		t.Fatal("the second half must play alongside the first")
	}
	if audible := s.AudibleTakes(); len(audible) != 2 {
		t.Fatalf("expected both halves audible, got %d", len(audible))
	}
}

func TestSplitTakeRejectsFramesOutsideTheTake(t *testing.T) {
	s := newSession(t)
	guitar := addTrack(t, s, "Guitar", 0)
	take := recordTake(t, s, guitar.ID(), "takes/take-001.wav", 0)

	for _, frame := range []int{0, 24000, 26000} {
		if _, err := s.SplitTake(take.ID(), frame, time.Unix(0, 0)); !session.IsCode(err, session.CodeInvalid) {
			t.Fatalf("expected frame %d to be refused, got %v", frame, err)
		}
	}
}

func TestDuplicateTakePastesALayeredCopy(t *testing.T) {
	s := newSession(t)
	guitar := addTrack(t, s, "Guitar", 0)
	take := recordTake(t, s, guitar.ID(), "takes/take-001.wav", 0)

	pasted, err := s.DuplicateTake(take.ID(), guitar.ID(), 30000, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if pasted.StartFrame() != 30000 || pasted.File() != take.File() {
		t.Fatalf("unexpected paste: %+v", pasted)
	}
	if !pasted.IsLayered() {
		t.Fatal("a paste on a track with an active take must be layered")
	}
	if audible := s.AudibleTakes(); len(audible) != 2 {
		t.Fatalf("expected the paste to play with the active take, got %d", len(audible))
	}
	if !s.TakeFileInUse(take.File()) {
		t.Fatal("the shared audio file must stay referenced")
	}
}

func TestMoveTakeRepositionsWithoutTouchingAudio(t *testing.T) {
	s := newSession(t)
	guitar := addTrack(t, s, "Guitar", 0)
	take := recordTake(t, s, guitar.ID(), "takes/take-001.wav", 0)

	if err := s.MoveTake(take.ID(), 48000); err != nil {
		t.Fatalf("move: %v", err)
	}
	moved, ok := s.FindTake(take.ID())
	if !ok || moved.StartFrame() != 48000 {
		t.Fatalf("expected the take at 48000, got %+v", moved)
	}
	if moved.File() != take.File() || moved.Trim() != take.Trim() {
		t.Fatal("moving must not touch the file or its trim")
	}
	if err := s.MoveTake(take.ID(), -1); !session.IsCode(err, session.CodeInvalid) {
		t.Fatalf("expected a negative position to be refused, got %v", err)
	}
}

func TestDuplicateTakeActivatesAnEmptyTrack(t *testing.T) {
	s := newSession(t)
	source := addTrack(t, s, "Guitar", 0)
	take := recordTake(t, s, source.ID(), "takes/take-001.wav", 0)
	empty := addTrack(t, s, "Mic", 1)

	pasted, err := s.DuplicateTake(take.ID(), empty.ID(), 0, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	reloaded, ok := s.FindTrack(empty.ID())
	if !ok || reloaded.ActiveTakeID() != pasted.ID() {
		t.Fatalf("expected the paste to become the active take, got %s", reloaded.ActiveTakeID())
	}
}
