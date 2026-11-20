package session

import (
	"strings"

	"gorecord/domain/mix"
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
	return Track{
		id:      id,
		name:    cleanName,
		source:  normalized,
		effects: mix.DefaultEffects(),
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

// InputChannel returns the interface channel, or -1 for a system audio track.
func (t Track) InputChannel() int {
	if t.source.IsSystem() {
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
	track.printEffects = state.PrintEffects && track.source.IsInput()
	track.effects = state.Effects.Normalize()
	track.activeTakeID = state.ActiveTakeID
	return track, nil
}

func validateInputChannel(channel int) error {
	if channel < 0 || channel > maxInputChannel {
		return invalid("input channel %d is outside 0..%d", channel, maxInputChannel)
	}
	return nil
}
