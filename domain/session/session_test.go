package session_test

import (
	"testing"
	"time"

	"fanamix/domain/session"
)

func newSession(t *testing.T) *session.Session {
	t.Helper()
	created, err := session.New("Riff idea", 48000, time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return created
}

func addTrack(t *testing.T, target *session.Session, name string, channel int) session.Track {
	t.Helper()
	track, err := target.AddTrack(name, channel)
	if err != nil {
		t.Fatalf("add track %s: %v", name, err)
	}
	return track
}

func TestArmPlanOrdersByInputChannel(t *testing.T) {
	s := newSession(t)
	microphone := addTrack(t, s, "Mic", 3)
	guitar := addTrack(t, s, "Guitar", 1)

	if err := s.SetArm(microphone.ID(), true); err != nil {
		t.Fatalf("arm microphone: %v", err)
	}
	if err := s.SetArm(guitar.ID(), true); err != nil {
		t.Fatalf("arm guitar: %v", err)
	}

	plan, err := s.ArmPlan()
	if err != nil {
		t.Fatalf("arm plan: %v", err)
	}
	if len(plan) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(plan))
	}
	if plan[0].Source.Channel != 1 || plan[1].Source.Channel != 3 {
		t.Fatalf("expected channels 1,3 got %d,%d", plan[0].Source.Channel, plan[1].Source.Channel)
	}
}

func TestArmingSameChannelTwiceIsRejected(t *testing.T) {
	s := newSession(t)
	first := addTrack(t, s, "Guitar", 0)
	second := addTrack(t, s, "Double", 0)

	if err := s.SetArm(first.ID(), true); err != nil {
		t.Fatalf("arm first: %v", err)
	}
	err := s.SetArm(second.ID(), true)
	if !session.IsCode(err, session.CodeConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestArmPlanRequiresAnArmedTrack(t *testing.T) {
	s := newSession(t)
	addTrack(t, s, "Guitar", 0)

	if _, err := s.ArmPlan(); !session.IsCode(err, session.CodeInvalid) {
		t.Fatalf("expected invalid arm plan, got %v", err)
	}
}

func TestAudibleTakesResolveSoloMuteAndActiveTake(t *testing.T) {
	s := newSession(t)
	guitar := addTrack(t, s, "Guitar", 0)
	voice := addTrack(t, s, "Voice", 1)

	firstTake := recordTake(t, s, guitar.ID(), "takes/take-001.wav", 0)
	secondTake := recordTake(t, s, guitar.ID(), "takes/take-002.wav", 48000)
	recordTake(t, s, voice.ID(), "takes/take-003.wav", 0)

	if audible := s.AudibleTakes(); len(audible) != 2 {
		t.Fatalf("expected 2 audible takes, got %d", len(audible))
	}
	if err := s.SetActiveTake(guitar.ID(), secondTake.ID()); err != nil {
		t.Fatalf("activate second take: %v", err)
	}
	if audible := s.AudibleTakes(); audible[0].Take.ID() != secondTake.ID() {
		t.Fatalf("expected second take audible, got %s", audible[0].Take.ID())
	}
	if err := s.SetSolo(voice.ID(), true); err != nil {
		t.Fatalf("solo voice: %v", err)
	}
	audible := s.AudibleTakes()
	if len(audible) != 1 || audible[0].Take.TrackID() != voice.ID() {
		t.Fatalf("solo should isolate the voice track, got %+v", audible)
	}
	if err := s.SetTakeMute(audible[0].Take.ID(), true); err != nil {
		t.Fatalf("mute take: %v", err)
	}
	if len(s.AudibleTakes()) != 0 {
		t.Fatal("muted take must not be audible")
	}
	_ = firstTake
}

func TestRemoveTakeReattachesAnotherTake(t *testing.T) {
	s := newSession(t)
	guitar := addTrack(t, s, "Guitar", 0)
	first := recordTake(t, s, guitar.ID(), "takes/take-001.wav", 0)
	second := recordTake(t, s, guitar.ID(), "takes/take-002.wav", 48000)

	if _, err := s.RemoveTake(second.ID()); err != nil {
		t.Fatalf("remove take: %v", err)
	}
	reloaded, ok := s.FindTrack(guitar.ID())
	if !ok {
		t.Fatal("track disappeared")
	}
	if reloaded.ActiveTakeID() != first.ID() {
		t.Fatalf("expected active take %s, got %s", first.ID(), reloaded.ActiveTakeID())
	}
}

func TestTakeRejectsMismatchedSampleRate(t *testing.T) {
	s := newSession(t)
	guitar := addTrack(t, s, "Guitar", 0)

	_, err := s.AddTake(guitar.ID(), "takes/take-001.wav", 0, 1, 44100, false, false, time.Unix(0, 0))
	if !session.IsCode(err, session.CodeInvalid) {
		t.Fatalf("expected invalid sample rate, got %v", err)
	}
}

func recordTake(t *testing.T, s *session.Session, trackID session.TrackID, file string, start int) session.Take {
	t.Helper()
	take, err := s.AddTake(trackID, file, start, 1, s.SampleRate(), false, false, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("add take: %v", err)
	}
	if err := s.CompleteTake(take.ID(), 24000); err != nil {
		t.Fatalf("complete take: %v", err)
	}
	return take
}
