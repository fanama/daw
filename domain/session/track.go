package session

import (
	"math"
	"strings"

	"fanamix/domain/mix"
)

// Track is one recorded part: an audio source with its own effect chain and an
// active take. Mutations belong to the Session aggregate.
type Track struct {
	id           TrackID
	name         string
	source       Source
	armed        bool
	muted        bool
	soloed       bool
	volumeDB     float64
	pan          float64
	printEffects bool
	effects      mix.Effects
	activeTakeID TakeID
}

// NewTrack builds a track bound to one interface channel.
func NewTrack(id TrackID, name string, inputChannel int) (Track, error) {
	return NewTrackWithSource(id, name, InputSource(inputChannel))
}

// NewTrackWithSource builds a track for any supported source.
func NewTrackWithSource(id TrackID, name string, source Source) (Track, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return Track{}, invalid("track name is required")
	}
	normalized, err := source.normalized()
	if err != nil {
		return Track{}, err
	}
	effects := mix.DefaultEffects()
	if normalized.IsSystem() || normalized.IsInstrument() {
		effects = mix.DryEffects()
	}
	return Track{
		id:      id,
		name:    cleanName,
		source:  normalized,
		effects: effects,
	}, nil
}

// ID returns the track identifier.
func (t Track) ID() TrackID { return t.id }

// Name returns the display name.
func (t Track) Name() string { return t.name }

// Source returns where the track records from.
func (t Track) Source() Source { return t.source }

// IsSystemAudio reports whether the track records the computer's output.
func (t Track) IsSystemAudio() bool { return t.source.IsSystem() }

// IsInstrument reports whether the track is played from notes instead of
// recorded from an input.
func (t Track) IsInstrument() bool { return t.source.IsInstrument() }

// Instrument returns the built-in sound of an instrument track.
func (t Track) Instrument() Instrument { return t.source.Instrument }

// InputChannel returns the interface channel, or -1 for non-input tracks.
func (t Track) InputChannel() int {
	if t.source.IsSystem() || t.source.IsInstrument() {
		return -1
	}
	return t.source.Channel
}

// IsArmed reports whether the track records on the next take.
func (t Track) IsArmed() bool { return t.armed }

// IsMuted reports whether the track is muted.
func (t Track) IsMuted() bool { return t.muted }

// IsSoloed reports whether the track participates in a solo group.
func (t Track) IsSoloed() bool { return t.soloed }

// VolumeDB returns the monitor fader level in decibels.
func (t Track) VolumeDB() float64 { return t.volumeDB }

// Pan returns the monitor position from -1 (left) to 1 (right).
func (t Track) Pan() float64 { return t.pan }

// PrintsEffects reports whether the take is written with effects baked in.
func (t Track) PrintsEffects() bool { return t.printEffects }

// Effects returns the effect chain configuration.
func (t Track) Effects() mix.Effects { return t.effects }

// ActiveTakeID returns the audible take, if one was chosen.
func (t Track) ActiveTakeID() TakeID { return t.activeTakeID }

// IsAudible resolves mute and solo across the session.
func (t Track) IsAudible(anySolo bool) bool {
	if t.muted {
		return false
	}
	return !anySolo || t.soloed
}

func (t Track) state() TrackState {
	return TrackState{
		ID:           t.id,
		Name:         t.name,
		InputChannel: t.InputChannel(),
		Source:       t.source,
		Armed:        t.armed,
		Muted:        t.muted,
		Soloed:       t.soloed,
		VolumeDB:     t.volumeDB,
		Pan:          t.pan,
		PrintEffects: t.printEffects,
		Effects:      t.effects,
		ActiveTakeID: t.activeTakeID,
	}
}

func trackFromState(state TrackState) (Track, error) {
	source := state.Source
	if source.Kind == "" {
		source = InputSource(state.InputChannel)
	}
	track, err := NewTrackWithSource(state.ID, state.Name, source)
	if err != nil {
		return Track{}, err
	}
	track.armed = state.Armed
	track.muted = state.Muted
	track.soloed = state.Soloed
	track.volumeDB = clampVolumeDB(state.VolumeDB)
	track.pan = clampPan(state.Pan)
	track.printEffects = state.PrintEffects && track.source.IsInput()
	track.effects = state.Effects.Normalize()
	track.activeTakeID = state.ActiveTakeID
	return track, nil
}

// Track volume is a monitor fader: it changes what is heard, never the take.
const (
	MinTrackVolumeDB = -60.0
	MaxTrackVolumeDB = 6.0
)

func clampVolumeDB(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Max(MinTrackVolumeDB, math.Min(MaxTrackVolumeDB, value))
}

// Track pan is a monitor control too: -1 is hard left, 1 hard right.
const (
	MinTrackPan = -1.0
	MaxTrackPan = 1.0
)

func clampPan(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Max(MinTrackPan, math.Min(MaxTrackPan, value))
}

func validateInputChannel(channel int) error {
	if channel < 0 || channel > maxInputChannel {
		return invalid("input channel %d is outside 0..%d", channel, maxInputChannel)
	}
	return nil
}
