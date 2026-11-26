package session

import (
	"fmt"
	"math"
	"strings"

	"fanamix/domain/mix"
)

// AddTrack appends a track bound to one interface channel.
func (s *Session) AddTrack(name string, inputChannel int) (Track, error) {
	return s.AddTrackWithSource(name, InputSource(inputChannel))
}

// AddTrackWithSource appends a track for any supported source.
func (s *Session) AddTrackWithSource(name string, source Source) (Track, error) {
	track, err := NewTrackWithSource(TrackID(fmt.Sprintf("trk_%d", s.nextTrack)), name, source)
	if err != nil {
		return Track{}, err
	}
	s.nextTrack++
	s.tracks = append(s.tracks, track)
	return track, nil
}

// RenameTrack changes a track's display name.
func (s *Session) RenameTrack(id TrackID, name string) error {
	index, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return invalid("track name is required")
	}
	s.tracks[index].name = cleanName
	return nil
}

// SetArm arms or disarms a track, enforcing unique input channels and at most
// one PC audio source per pass.
func (s *Session) SetArm(id TrackID, armed bool) error {
	index, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	if armed {
		if err := s.ensureArmable(index); err != nil {
			return err
		}
	}
	s.tracks[index].armed = armed
	return nil
}

// SetMute mutes or unmutes a track.
func (s *Session) SetMute(id TrackID, muted bool) error {
	index, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	s.tracks[index].muted = muted
	return nil
}

// SetSolo puts a track in or out of the solo group.
func (s *Session) SetSolo(id TrackID, soloed bool) error {
	index, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	s.tracks[index].soloed = soloed
	return nil
}

// SetVolume sets a track's monitor fader in decibels.
func (s *Session) SetVolume(id TrackID, volumeDB float64) error {
	index, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	if math.IsNaN(volumeDB) || math.IsInf(volumeDB, 0) {
		return invalid("track volume must be finite")
	}
	s.tracks[index].volumeDB = clampVolumeDB(volumeDB)
	return nil
}

// SetPan places a track in the stereo field from -1 (left) to 1 (right).
func (s *Session) SetPan(id TrackID, pan float64) error {
	index, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	if math.IsNaN(pan) || math.IsInf(pan, 0) {
		return invalid("track pan must be finite")
	}
	s.tracks[index].pan = clampPan(pan)
	return nil
}

// SetInstrument changes the built-in sound of an instrument track.
func (s *Session) SetInstrument(id TrackID, instrument Instrument) error {
	index, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	if !s.tracks[index].source.IsInstrument() {
		return invalid("track %q is not an instrument track", s.tracks[index].name)
	}
	normalized, err := instrument.normalized()
	if err != nil {
		return err
	}
	s.tracks[index].source.Instrument = normalized
	return nil
}

// SetSource changes where a track records from.
func (s *Session) SetSource(id TrackID, source Source) error {
	index, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	normalized, err := source.normalized()
	if err != nil {
		return err
	}
	if s.tracks[index].source.IsInstrument() || normalized.IsInstrument() {
		return invalid("instrument tracks keep their instrument source")
	}
	previous := s.tracks[index].source
	s.tracks[index].source = normalized
	if normalized.IsSystem() {
		s.tracks[index].printEffects = false
	}
	if s.tracks[index].armed {
		if err := s.ensureArmable(index); err != nil {
			s.tracks[index].source = previous
			return err
		}
	}
	return nil
}

// SetInputChannel moves a track to another interface channel.
func (s *Session) SetInputChannel(id TrackID, channel int) error {
	return s.SetSource(id, InputSource(channel))
}

// SetPrintEffects toggles baking effects into the recorded file.
func (s *Session) SetPrintEffects(id TrackID, print bool) error {
	index, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	if s.tracks[index].source.IsSystem() {
		return invalid("PC audio takes are always recorded dry")
	}
	s.tracks[index].printEffects = print
	return nil
}

// SetEffects replaces a track's chain after normalizing every parameter.
func (s *Session) SetEffects(id TrackID, effects mix.Effects) error {
	index, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	s.tracks[index].effects = effects.Normalize()
	return nil
}

// AddEffect appends an enabled instance of kind to the track's chain.
func (s *Session) AddEffect(id TrackID, kind mix.EffectKind) error {
	index, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	if !mix.ValidEffectKind(kind) {
		return invalid("unknown effect %q", kind)
	}
	chain := s.tracks[index].effects.Chain
	if len(chain) >= mix.MaxEffectInstances {
		return invalid("a track can hold at most %d effects", mix.MaxEffectInstances)
	}
	updated := make([]mix.Effect, 0, len(chain)+1)
	updated = append(updated, chain...)
	updated = append(updated, mix.NewEffect(kind))
	s.tracks[index].effects = mix.Effects{Chain: updated}
	return nil
}

// RemoveEffect drops the instance at index from the track's chain.
func (s *Session) RemoveEffect(id TrackID, index int) error {
	trackIndex, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	chain := s.tracks[trackIndex].effects.Chain
	if index < 0 || index >= len(chain) {
		return invalid("effect %d does not exist", index)
	}
	updated := make([]mix.Effect, 0, len(chain)-1)
	updated = append(updated, chain[:index]...)
	updated = append(updated, chain[index+1:]...)
	s.tracks[trackIndex].effects = mix.Effects{Chain: updated}
	return nil
}

// MoveEffect moves the instance at from so it sits at to, keeping the rest of
// the chain in order.
func (s *Session) MoveEffect(id TrackID, from, to int) error {
	trackIndex, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	chain := s.tracks[trackIndex].effects.Chain
	if from < 0 || from >= len(chain) || to < 0 || to >= len(chain) {
		return invalid("cannot move effect %d to %d", from, to)
	}
	if from == to {
		return nil
	}
	updated := make([]mix.Effect, 0, len(chain))
	for index, effect := range chain {
		if index != from {
			updated = append(updated, effect)
		}
	}
	updated = append(updated, mix.Effect{})
	copy(updated[to+1:], updated[to:])
	updated[to] = chain[from]
	s.tracks[trackIndex].effects = mix.Effects{Chain: updated}
	return nil
}

// RemoveTrack deletes a track and returns its takes so the caller can move
// the audio to the trash instead of destroying it.
func (s *Session) RemoveTrack(id TrackID) ([]Take, error) {
	index, err := s.trackIndex(id)
	if err != nil {
		return nil, err
	}
	s.tracks = append(s.tracks[:index], s.tracks[index+1:]...)
	kept := make([]Take, 0, len(s.takes))
	var orphaned []Take
	for _, take := range s.takes {
		if take.trackID == id {
			orphaned = append(orphaned, take)
			continue
		}
		kept = append(kept, take)
	}
	s.takes = kept
	keptNotes := make([]Note, 0, len(s.notes))
	for _, note := range s.notes {
		if note.trackID != id {
			keptNotes = append(keptNotes, note)
		}
	}
	s.notes = keptNotes
	return orphaned, nil
}

func (s *Session) ensureArmable(index int) error {
	candidate := s.tracks[index]
	if candidate.source.IsInstrument() {
		return invalid("instrument tracks are not recorded")
	}
	for other := range s.tracks {
		if other == index || !s.tracks[other].armed {
			continue
		}
		if candidate.source.IsSystem() || s.tracks[other].source.IsSystem() {
			if candidate.source.IsSystem() && s.tracks[other].source.IsSystem() {
				return conflict("only one PC audio track can be armed")
			}
			continue
		}
		if s.tracks[other].source.Channel == candidate.source.Channel {
			return conflict("input channel %d is already armed by %q", candidate.source.Channel, s.tracks[other].name)
		}
	}
	return nil
}

func (s *Session) trackIndex(id TrackID) (int, error) {
	for index := range s.tracks {
		if s.tracks[index].id == id {
			return index, nil
		}
	}
	return -1, notFound("track %s does not exist", id)
}
