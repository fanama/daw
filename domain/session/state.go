package session

import (
	"time"

	"gorecord/domain/mix"
)

// TrackState is the persistence- and transport-neutral shape of a track.
// Adapters map their own records to and from it.
type TrackState struct {
	ID           TrackID
	Name         string
	InputChannel int
	Source       Source
	Armed        bool
	Muted        bool
	Soloed       bool
	VolumeDB     float64
	PrintEffects bool
	Effects      mix.Effects
	ActiveTakeID TakeID
}

// TakeState is the persistence- and transport-neutral shape of a take.
type TakeState struct {
	ID           TakeID
	TrackID      TrackID
	File         string
	StartFrame   int
	LengthFrames int
	Channels     int
	SampleRate   int
	Printed      bool
	Incomplete   bool
	Muted        bool
	Trim         Trim
	GainDB       float64
	CreatedAt    time.Time
}

// State is a complete snapshot of a session aggregate, used to restore it
// after loading and to export it for persistence.
type State struct {
	Name          string
	CreatedAt     time.Time
	SampleRate    int
	Tracks        []TrackState
	Takes         []TakeState
	Click         mix.Click
	LatencyOffset int
	NextTrack     int
	NextTake      int
}

// State exports the aggregate without exposing mutable internals.
func (s *Session) State() State {
	tracks := make([]TrackState, 0, len(s.tracks))
	for _, track := range s.tracks {
		tracks = append(tracks, track.state())
	}
	takes := make([]TakeState, 0, len(s.takes))
	for _, take := range s.takes {
		takes = append(takes, take.state())
	}
	return State{
		Name:          s.name,
		CreatedAt:     s.createdAt,
		SampleRate:    s.sampleRate,
		Tracks:        tracks,
		Takes:         takes,
		Click:         s.click,
		LatencyOffset: s.latencyOffset,
		NextTrack:     s.nextTrack,
		NextTake:      s.nextTake,
	}
}
