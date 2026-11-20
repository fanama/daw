package session_test

import (
	"testing"
	"time"

	"gorecord/domain/mix"
	"gorecord/domain/session"
)

func TestCapacityLevelsFollowTheCeiling(t *testing.T) {
	channels := 4
	limit := session.TakeFrameLimit(channels)
	if limit <= 0 {
		t.Fatalf("expected a positive frame limit, got %d", limit)
	}
	cases := []struct {
		written int
		want    session.CapacityLevel
	}{
		{0, session.CapacityOK},
		{limit * 74 / 100, session.CapacityOK},
		{limit * 75 / 100, session.CapacityWarn},
		{limit * 92 / 100, session.CapacityCritical},
		{limit * 96 / 100, session.CapacityFull},
	}
	for _, testCase := range cases {
		if got := session.CapacityFor(testCase.written, channels); got != testCase.want {
			t.Fatalf("written %d: expected level %v, got %v", testCase.written, testCase.want, got)
		}
	}
}

func TestCapacityRemainingDurationShrinksWithChannels(t *testing.T) {
	mono := session.RemainingDuration(0, 1, 48000)
	stereo := session.RemainingDuration(0, 2, 48000)
	if mono <= stereo {
		t.Fatalf("expected mono headroom above stereo, got %v and %v", mono, stereo)
	}
	if session.RemainingFrames(session.TakeFrameLimit(2), 2) != 0 {
		t.Fatal("a full take must report zero remaining frames")
	}
}

func TestEffectsNormalizeClampsEveryParameter(t *testing.T) {
	effects := mix.Effects{
		Amp:    mix.Amp{Drive: 4, BassDB: -99, MidDB: 99, LevelDB: 50},
		Cab:    mix.Cab{Character: -1},
		EQ:     mix.EQ{Bands: [3]mix.EQBand{{FrequencyHz: 5, Q: 100, GainDB: 90}}},
		Comp:   mix.Compressor{ThresholdDB: -200, Ratio: 50, AttackMs: 0, ReleaseMs: 1, KneeDB: 40, MakeupDB: 90},
		Reverb: mix.Reverb{Size: 9, Damp: -3, Mix: 3, PreDelayMs: 900},
	}
	normalized := effects.Normalize()

	if normalized.Amp.Drive != 1 || normalized.Amp.BassDB != -12 || normalized.Amp.MidDB != 12 {
		t.Fatalf("amp not clamped: %+v", normalized.Amp)
	}
	if normalized.Cab.Character != 0 {
		t.Fatalf("cab not clamped: %+v", normalized.Cab)
	}
	if normalized.EQ.Bands[0].FrequencyHz != 20 || normalized.EQ.Bands[0].GainDB != 24 {
		t.Fatalf("eq not clamped: %+v", normalized.EQ.Bands[0])
	}
	if normalized.Comp.Ratio != 20 || normalized.Comp.ReleaseMs != 10 {
		t.Fatalf("compressor not clamped: %+v", normalized.Comp)
	}
	if normalized.Reverb.Mix != 1 || normalized.Reverb.PreDelayMs != 100 {
		t.Fatalf("reverb not clamped: %+v", normalized.Reverb)
	}
}

func TestClickNormalizeBoundsTempo(t *testing.T) {
	click := mix.Click{BPM: 900, BeatsPerBar: 0, CountInBars: 9}.Normalize()
	if click.BPM != 300 || click.BeatsPerBar != 2 || click.CountInBars != 2 {
		t.Fatalf("unexpected click normalization: %+v", click)
	}
	if frames := click.FramesPerBeat(48000); frames != 9600 {
		t.Fatalf("expected 9600 frames per beat at 300 BPM, got %d", frames)
	}
}

func TestRestoreRevalidatesInvariants(t *testing.T) {
	state := session.State{
		Name:       "Riff idea",
		CreatedAt:  time.Unix(0, 0),
		SampleRate: 48000,
		Tracks: []session.TrackState{
			{ID: "trk_1", Name: "Guitar", InputChannel: 0, Armed: true},
			{ID: "trk_2", Name: "Double", InputChannel: 0, Armed: true},
		},
	}
	if _, err := session.Restore(state); !session.IsCode(err, session.CodeConflict) {
		t.Fatalf("expected conflict for doubly armed channel, got %v", err)
	}
}
