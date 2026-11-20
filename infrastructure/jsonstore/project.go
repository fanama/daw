package jsonstore

import (
	"errors"
	"time"

	"gorecord/domain/mix"
	"gorecord/domain/session"
)

// SchemaVersion is the project file version this build writes.
const SchemaVersion = 1

// ErrUnsupportedSchema is returned when a project was written by a newer app.
var ErrUnsupportedSchema = errors.New("jsonstore: project schema is newer than this app")

type appJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type clickJSON struct {
	Enabled     bool    `json:"enabled"`
	BPM         float64 `json:"bpm"`
	BeatsPerBar int     `json:"beatsPerBar"`
	CountInBars int     `json:"countInBars"`
}

type sessionJSON struct {
	Name                string    `json:"name"`
	CreatedAt           time.Time `json:"createdAt"`
	SampleRate          int       `json:"sampleRate"`
	Click               clickJSON `json:"click"`
	RecordLatencyOffset int       `json:"recordLatencyOffset"`
	NextTrack           int       `json:"nextTrack"`
	NextTake            int       `json:"nextTake"`
}

type sourceJSON struct {
	Kind    string `json:"kind"`
	Channel int    `json:"channel"`
}

type trackJSON struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	InputChannel int         `json:"inputChannel"`
	Source       sourceJSON  `json:"source"`
	Armed        bool        `json:"armed"`
	Mute         bool        `json:"mute"`
	Solo         bool        `json:"solo"`
	PrintEffects bool        `json:"printEffects"`
	Effects      effectsJSON `json:"effects"`
	ActiveTakeID string      `json:"activeTakeId"`
}

type trimJSON struct {
	StartFrame int `json:"startFrame"`
	EndFrame   int `json:"endFrame"`
}

type takeJSON struct {
	ID           string    `json:"id"`
	TrackID      string    `json:"trackId"`
	File         string    `json:"file"`
	StartFrame   int       `json:"startFrame"`
	LengthFrames int       `json:"lengthFrames"`
	Channels     int       `json:"channels"`
	SampleRate   int       `json:"sampleRate"`
	Format       string    `json:"format"`
	Printed      bool      `json:"printed"`
	Incomplete   bool      `json:"incomplete"`
	Muted        bool      `json:"muted"`
	Trim         trimJSON  `json:"trim"`
	GainDB       float64   `json:"gainDb"`
	CreatedAt    time.Time `json:"createdAt"`
}

type projectJSON struct {
	SchemaVersion int         `json:"schemaVersion"`
	App           appJSON     `json:"app"`
	Session       sessionJSON `json:"session"`
	Tracks        []trackJSON `json:"tracks"`
	Takes         []takeJSON  `json:"takes"`
}

func projectFromState(state session.State) projectJSON {
	return projectJSON{
		SchemaVersion: SchemaVersion,
		App:           appJSON{Name: "gorecord", Version: "0.1.0"},
		Session:       sessionFromState(state),
		Tracks:        tracksFromState(state.Tracks),
		Takes:         takesFromState(state.Takes),
	}
}

func sessionFromState(state session.State) sessionJSON {
	return sessionJSON{
		Name:                state.Name,
		CreatedAt:           state.CreatedAt,
		SampleRate:          state.SampleRate,
		Click:               clickFromDomain(state.Click),
		RecordLatencyOffset: state.LatencyOffset,
		NextTrack:           state.NextTrack,
		NextTake:            state.NextTake,
	}
}

func tracksFromState(tracks []session.TrackState) []trackJSON {
	stored := make([]trackJSON, 0, len(tracks))
	for _, track := range tracks {
		stored = append(stored, trackFromState(track))
	}
	return stored
}

func trackFromState(track session.TrackState) trackJSON {
	return trackJSON{
		ID:           track.ID.String(),
		Name:         track.Name,
		InputChannel: track.InputChannel,
		Source:       sourceJSON{Kind: string(track.Source.Kind), Channel: track.Source.Channel},
		Armed:        track.Armed,
		Mute:         track.Muted,
		Solo:         track.Soloed,
		PrintEffects: track.PrintEffects,
		Effects:      effectsFromDomain(track.Effects),
		ActiveTakeID: track.ActiveTakeID.String(),
	}
}

func takesFromState(takes []session.TakeState) []takeJSON {
	stored := make([]takeJSON, 0, len(takes))
	for _, take := range takes {
		stored = append(stored, takeFromState(take))
	}
	return stored
}

func takeFromState(take session.TakeState) takeJSON {
	return takeJSON{
		ID:           take.ID.String(),
		TrackID:      take.TrackID.String(),
		File:         take.File,
		StartFrame:   take.StartFrame,
		LengthFrames: take.LengthFrames,
		Channels:     take.Channels,
		SampleRate:   take.SampleRate,
		Format:       "f32le",
		Printed:      take.Printed,
		Incomplete:   take.Incomplete,
		Muted:        take.Muted,
		Trim:         trimJSON{StartFrame: take.Trim.StartFrame, EndFrame: take.Trim.EndFrame},
		GainDB:       take.GainDB,
		CreatedAt:    take.CreatedAt,
	}
}

func stateFromProject(project projectJSON) (session.State, error) {
	if project.SchemaVersion > SchemaVersion {
		return session.State{}, ErrUnsupportedSchema
	}
	return session.State{
		Name:          project.Session.Name,
		CreatedAt:     project.Session.CreatedAt,
		SampleRate:    project.Session.SampleRate,
		Tracks:        tracksToState(project.Tracks),
		Takes:         takesToState(project.Takes),
		Click:         clickToDomain(project.Session.Click),
		LatencyOffset: project.Session.RecordLatencyOffset,
		NextTrack:     project.Session.NextTrack,
		NextTake:      project.Session.NextTake,
	}, nil
}

func tracksToState(tracks []trackJSON) []session.TrackState {
	restored := make([]session.TrackState, 0, len(tracks))
	for _, track := range tracks {
		source := session.Source{Kind: session.SourceKind(track.Source.Kind), Channel: track.Source.Channel}
		if source.Kind == "" {
			source = session.InputSource(track.InputChannel)
		}
		restored = append(restored, session.TrackState{
			ID:           session.TrackID(track.ID),
			Name:         track.Name,
			InputChannel: track.InputChannel,
			Source:       source,
			Armed:        track.Armed,
			Muted:        track.Mute,
			Soloed:       track.Solo,
			PrintEffects: track.PrintEffects,
			Effects:      effectsToDomain(track.Effects),
			ActiveTakeID: session.TakeID(track.ActiveTakeID),
		})
	}
	return restored
}

func takesToState(takes []takeJSON) []session.TakeState {
	restored := make([]session.TakeState, 0, len(takes))
	for _, take := range takes {
		restored = append(restored, session.TakeState{
			ID:           session.TakeID(take.ID),
			TrackID:      session.TrackID(take.TrackID),
			File:         take.File,
			StartFrame:   take.StartFrame,
			LengthFrames: take.LengthFrames,
			Channels:     take.Channels,
			SampleRate:   take.SampleRate,
			Printed:      take.Printed,
			Incomplete:   take.Incomplete,
			Muted:        take.Muted,
			Trim:         session.Trim{StartFrame: take.Trim.StartFrame, EndFrame: take.Trim.EndFrame},
			GainDB:       take.GainDB,
			CreatedAt:    take.CreatedAt,
		})
	}
	return restored
}

func clickFromDomain(click mix.Click) clickJSON {
	return clickJSON{
		Enabled:     click.Enabled,
		BPM:         click.BPM,
		BeatsPerBar: click.BeatsPerBar,
		CountInBars: click.CountInBars,
	}
}

func clickToDomain(click clickJSON) mix.Click {
	return mix.Click{
		Enabled:     click.Enabled,
		BPM:         click.BPM,
		BeatsPerBar: click.BeatsPerBar,
		CountInBars: click.CountInBars,
	}
}
