package session

import "fanamix/domain/mix"

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
