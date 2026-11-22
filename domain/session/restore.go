package session

// Restore rebuilds a session from persisted state while re-checking every
// invariant, so a hand-edited or older project file cannot corrupt the model.
func Restore(state State) (*Session, error) {
	restored, err := New(state.Name, state.SampleRate, state.CreatedAt)
	if err != nil {
		return nil, err
	}
	restored.click = state.Click.Normalize()
	restored.latencyOffset = state.LatencyOffset
	restored.nextTrack = max(state.NextTrack, 1)
	restored.nextTake = max(state.NextTake, 1)
	restored.nextMarker = max(state.NextMarker, 1)

	for _, trackState := range state.Tracks {
		track, trackErr := trackFromState(trackState)
		if trackErr != nil {
			return nil, trackErr
		}
		restored.tracks = append(restored.tracks, track)
	}
	for _, takeState := range state.Takes {
		take, takeErr := takeFromState(takeState)
		if takeErr != nil {
			return nil, takeErr
		}
		restored.takes = append(restored.takes, take)
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

func (s *Session) validateArming() error {
	armedChannels := map[int]TrackID{}
	systemArmed := TrackID("")
	for _, track := range s.tracks {
		if !track.armed {
			continue
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
