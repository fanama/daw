package session

// Restore rebuilds a session from persisted state while re-checking every
// invariant, so a hand-edited or older project file cannot corrupt the model.
func Restore(state State) (*Session, error) {
	restored, err := baseFromState(state)
	if err != nil {
		return nil, err
	}
	if err := restored.loadTracks(state.Tracks); err != nil {
		return nil, err
	}
	if err := restored.loadTakes(state.Takes); err != nil {
		return nil, err
	}
	if err := restored.loadNotes(state.Notes); err != nil {
		return nil, err
	}
	markers, err := markersFromState(state.Markers)
	if err != nil {
		return nil, err
	}
	restored.markers = markers
	if err := restored.validate(); err != nil {
		return nil, err
	}
	return restored, nil
}

func baseFromState(state State) (*Session, error) {
	restored, err := New(state.Name, state.SampleRate, state.CreatedAt)
	if err != nil {
		return nil, err
	}
	restored.click = state.Click.Normalize()
	restored.latencyOffset = state.LatencyOffset
	restored.nextTrack = max(state.NextTrack, 1)
	restored.nextTake = max(state.NextTake, 1)
	restored.nextNote = max(state.NextNote, 1)
	restored.nextMarker = max(state.NextMarker, 1)
	return restored, nil
}

func (s *Session) loadTracks(states []TrackState) error {
	for _, trackState := range states {
		track, err := trackFromState(trackState)
		if err != nil {
			return err
		}
		s.tracks = append(s.tracks, track)
	}
	return nil
}

func (s *Session) loadTakes(states []TakeState) error {
	for _, takeState := range states {
		take, err := takeFromState(takeState)
		if err != nil {
			return err
		}
		s.takes = append(s.takes, take)
	}
	return nil
}

func (s *Session) loadNotes(states []NoteState) error {
	for _, noteState := range states {
		note, err := noteFromState(noteState)
		if err != nil {
			return err
		}
		s.notes = append(s.notes, note)
	}
	return nil
}

func (s *Session) validate() error {
	trackIDs := map[TrackID]bool{}
	for _, track := range s.tracks {
		if trackIDs[track.id] {
			return conflict("track %s appears twice", track.id)
		}
		trackIDs[track.id] = true
	}
	if err := s.validateArming(); err != nil {
		return err
	}
	if err := s.validateTakes(trackIDs); err != nil {
		return err
	}
	return s.validateNotes(trackIDs)
}

func (s *Session) validateTakes(trackIDs map[TrackID]bool) error {
	seenTakes := map[TakeID]bool{}
	for _, take := range s.takes {
		if seenTakes[take.id] {
			return conflict("take %s appears twice", take.id)
		}
		seenTakes[take.id] = true
		if !trackIDs[take.trackID] {
			return invalid("take %s references unknown track %s", take.id, take.trackID)
		}
	}
	for _, track := range s.tracks {
		if track.activeTakeID.IsZero() {
			continue
		}
		if !seenTakes[track.activeTakeID] {
			return invalid("track %s references unknown take %s", track.id, track.activeTakeID)
		}
	}
	return nil
}

func (s *Session) validateNotes(trackIDs map[TrackID]bool) error {
	seen := map[NoteID]bool{}
	for _, note := range s.notes {
		if seen[note.id] {
			return conflict("note %s appears twice", note.id)
		}
		seen[note.id] = true
		if !trackIDs[note.trackID] {
			return invalid("note %s references unknown track %s", note.id, note.trackID)
		}
		for _, track := range s.tracks {
			if track.id == note.trackID && !track.source.IsInstrument() {
				return invalid("note %s belongs to %q, which is not an instrument track", note.id, track.name)
			}
		}
	}
	return nil
}

func (s *Session) validateArming() error {
	armedChannels := map[int]TrackID{}
	systemArmed := TrackID("")
	for _, track := range s.tracks {
		if !track.armed {
			continue
		}
		if track.source.IsInstrument() {
			return invalid("instrument track %s cannot be armed", track.id)
		}
		if track.source.IsSystem() {
			if !systemArmed.IsZero() {
				return conflict("only one PC audio track can be armed")
			}
			systemArmed = track.id
			continue
		}
		if owner, taken := armedChannels[track.source.Channel]; taken {
			return conflict("input channel %d is armed by both %s and %s", track.source.Channel, owner, track.id)
		}
		armedChannels[track.source.Channel] = track.id
	}
	return nil
}
