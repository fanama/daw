package session_test

import (
	"testing"
	"time"

	"fanamix/domain/mix"
	"fanamix/domain/session"
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

func TestSystemAudioArmsWithInputs(t *testing.T) {
	s := newSession(t)
	guitar := addTrack(t, s, "Guitar", 0)
	pc := addSystemTrack(t, s, "Browser")

	if err := s.SetArm(guitar.ID(), true); err != nil {
		t.Fatalf("arm guitar: %v", err)
	}
	if err := s.SetArm(pc.ID(), true); err != nil {
		t.Fatalf("arm PC audio: %v", err)
	}
	plan, err := s.ArmPlan()
	if err != nil {
		t.Fatalf("arm plan: %v", err)
	}
	if len(plan) != 2 || plan[0].Source.IsSystem() || !plan[1].Source.IsSystem() {
		t.Fatalf("expected input then PC audio, got %+v", plan)
	}
	if s.ArmedChannelCount() != 3 {
		t.Fatalf("expected mono input plus stereo PC audio, got %d channels", s.ArmedChannelCount())
	}
}

func TestOnlyOneSystemAudioTrackCanBeArmed(t *testing.T) {
	s := newSession(t)
	first := addSystemTrack(t, s, "Browser")
	second := addSystemTrack(t, s, "Player")

	if err := s.SetArm(first.ID(), true); err != nil {
		t.Fatalf("arm first PC track: %v", err)
	}
	if err := s.SetArm(second.ID(), true); !session.IsCode(err, session.CodeConflict) {
		t.Fatalf("expected conflict for a second PC track, got %v", err)
	}
}

func TestSystemAudioTakesAreAlwaysDry(t *testing.T) {
	s := newSession(t)
	pc := addSystemTrack(t, s, "Browser")

	if err := s.SetPrintEffects(pc.ID(), true); !session.IsCode(err, session.CodeInvalid) {
		t.Fatalf("expected invalid print request, got %v", err)
	}
}

func TestSystemAudioAcceptsChainedEffects(t *testing.T) {
	s := newSession(t)
	pc := addSystemTrack(t, s, "Browser")
	if !pc.Effects().Bypassed() {
		t.Fatalf("a new PC audio track should start dry: %+v", pc.Effects())
	}
	effects := mix.DryEffects()
	effects.EQ.Enabled = true
	effects.EQ.Bands[2].GainDB = 6
	if err := s.SetEffects(pc.ID(), effects); err != nil {
		t.Fatalf("set system effects: %v", err)
	}
	track, ok := s.FindTrack(pc.ID())
	if !ok || !track.Effects().EQ.Enabled || track.Effects().EQ.Bands[2].GainDB != 6 {
		t.Fatalf("system effects were not stored: %+v", track.Effects())
	}
}

func TestRestoreKeepsSystemAndInputArmedTogether(t *testing.T) {
	state := session.State{
		Name:       "PC idea",
		SampleRate: 48000,
		CreatedAt:  time.Unix(1_700_000_000, 0),
		Tracks: []session.TrackState{
			{ID: "trk_1", Name: "Guitar", InputChannel: 0, Source: session.InputSource(0), Armed: true},
			{ID: "trk_2", Name: "Browser", InputChannel: -1, Source: session.SystemSource(), Armed: true},
		},
	}
	restored, err := session.Restore(state)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	tracks := restored.Tracks()
	if len(tracks) != 2 || !tracks[0].IsArmed() || !tracks[1].IsArmed() {
		t.Fatalf("mixed arming was not restored: %+v", tracks)
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
