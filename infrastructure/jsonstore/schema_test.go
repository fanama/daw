package jsonstore

import (
	"testing"
	"time"

	"fanamix/domain/mix"
)

func TestStateFromProjectDriesLegacySystemEffects(t *testing.T) {
	project := projectJSON{
		SchemaVersion: 3,
		Session: sessionJSON{
			Name:       "PC idea",
			SampleRate: 48000,
			CreatedAt:  time.Unix(0, 0),
		},
		Tracks: []trackJSON{{
			ID:           "trk_1",
			Name:         "Browser",
			InputChannel: -1,
			Source:       sourceJSON{Kind: "system", Channel: -1},
			Effects:      effectsFromDomain(mix.DefaultEffects()),
		}},
	}
	state, err := stateFromProject(project)
	if err != nil {
		t.Fatalf("decode v3: %v", err)
	}
	if !state.Tracks[0].Effects.Bypassed() {
		t.Fatalf("legacy PC audio kept its default guitar chain: %+v", state.Tracks[0].Effects)
	}

	project.SchemaVersion = 4
	state, err = stateFromProject(project)
	if err != nil {
		t.Fatalf("decode v4: %v", err)
	}
	if !state.Tracks[0].Effects.Amp.Enabled {
		t.Fatalf("v4 PC audio effects were dropped: %+v", state.Tracks[0].Effects)
	}
}
