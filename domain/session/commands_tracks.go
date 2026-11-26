package session

import (
	"fmt"
	"math"
	"strings"
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

// SetRepeat sets how many times an instrument pattern loops after it. The
// notes stay untouched; repetition is a property of the track.
func (s *Session) SetRepeat(id TrackID, repeat int) error {
	index, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	if !s.tracks[index].source.IsInstrument() {
		return invalid("track %q is not an instrument track", s.tracks[index].name)
	}
	if repeat < 0 || repeat > MaxTrackRepeat {
		return invalid("repeat count %d is outside 0..%d", repeat, MaxTrackRepeat)
	}
	s.tracks[index].repeat = repeat
	return nil
}

// MaxPatternMinutes bounds how far an instrument pattern can be widened.
const MaxPatternMinutes = 30

// SetPatternFrames widens an instrument pattern past its last note. Zero goes
// back to the length the notes span, and a pattern can never be shorter than
// its notes.
func (s *Session) SetPatternFrames(id TrackID, frames int) error {
	index, err := s.trackIndex(id)
	if err != nil {
		return err
	}
	if !s.tracks[index].source.IsInstrument() {
		return invalid("track %q is not an instrument track", s.tracks[index].name)
	}
	if frames < 0 {
		return invalid("pattern length must not be negative")
	}
	if span := s.noteSpanFrames(id); frames > 0 && frames < span {
		return invalid("pattern length %d does not cover its notes (%d frames)", frames, span)
	}
	if limit := s.sampleRate * 60 * MaxPatternMinutes; frames > limit {
		return invalid("pattern length %d is above %d frames", frames, limit)
	}
	s.tracks[index].patternFrames = frames
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
