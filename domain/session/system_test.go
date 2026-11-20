package session_test

import (
	"testing"
	"time"

	"gorecord/domain/session"
)

func addSystemTrack(t *testing.T, target *session.Session, name string) session.Track {
	t.Helper()
	track, err := target.AddTrackWithSource(name, session.SystemSource())
	if err != nil {
		t.Fatalf("add system track %s: %v", name, err)
	}
	return track
}

func TestSystemAudioArmsAloneAndPlansStereo(t *testing.T) {
	s := newSession(t)
	pc := addSystemTrack(t, s, "Browser")

	if err := s.SetArm(pc.ID(), true); err != nil {
		t.Fatalf("arm PC audio: %v", err)
	}
	plan, err := s.ArmPlan()
	if err != nil {
		t.Fatalf("arm plan: %v", err)
	}
	if len(plan) != 1 || !plan[0].Source.IsSystem() {
		t.Fatalf("expected one system slot, got %+v", plan)
	}
	if s.ArmedChannelCount() != 2 {
		t.Fatalf("system audio must occupy two channels, got %d", s.ArmedChannelCount())
	}
	if pc.InputChannel() != -1 {
		t.Fatalf("system track must not expose an input channel, got %d", pc.InputChannel())
	}
}

func TestSystemAudioCannotArmWithInputs(t *testing.T) {
	s := newSession(t)
	guitar := addTrack(t, s, "Guitar", 0)
	pc := addSystemTrack(t, s, "Browser")

	if err := s.SetArm(guitar.ID(), true); err != nil {
		t.Fatalf("arm guitar: %v", err)
	}
	if err := s.SetArm(pc.ID(), true); !session.IsCode(err, session.CodeConflict) {
		t.Fatalf("expected conflict arming PC audio over inputs, got %v", err)
	}
	if err := s.SetArm(guitar.ID(), false); err != nil {
		t.Fatalf("disarm guitar: %v", err)
	}
	if err := s.SetArm(pc.ID(), true); err != nil {
		t.Fatalf("arm PC audio: %v", err)
	}
	if err := s.SetArm(guitar.ID(), true); !session.IsCode(err, session.CodeConflict) {
		t.Fatalf("expected conflict arming inputs over PC audio, got %v", err)
	}
}

func TestSystemAudioTakesAreAlwaysDry(t *testing.T) {
	s := newSession(t)
	pc := addSystemTrack(t, s, "Browser")

	if err := s.SetPrintEffects(pc.ID(), true); !session.IsCode(err, session.CodeInvalid) {
		t.Fatalf("expected invalid print request, got %v", err)
	}
}

func TestRestoreRejectsSystemAndInputArmedTogether(t *testing.T) {
	state := session.State{
		Name:       "PC idea",
		SampleRate: 48000,
		CreatedAt:  time.Unix(1_700_000_000, 0),
		Tracks: []session.TrackState{
			{ID: "trk_1", Name: "Guitar", InputChannel: 0, Source: session.InputSource(0), Armed: true},
			{ID: "trk_2", Name: "Browser", InputChannel: -1, Source: session.SystemSource(), Armed: true},
		},
	}
	if _, err := session.Restore(state); !session.IsCode(err, session.CodeConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestRestoreKeepsSystemSourceAlone(t *testing.T) {
	state := session.State{
		Name:       "PC idea",
		SampleRate: 48000,
		CreatedAt:  time.Unix(1_700_000_000, 0),
		Tracks: []session.TrackState{
			{ID: "trk_1", Name: "Browser", InputChannel: -1, Source: session.SystemSource(), Armed: true},
		},
	}
	restored, err := session.Restore(state)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	tracks := restored.Tracks()
	if len(tracks) != 1 || !tracks[0].IsSystemAudio() || !tracks[0].IsArmed() {
		t.Fatalf("system source was not restored: %+v", tracks)
	}
}
