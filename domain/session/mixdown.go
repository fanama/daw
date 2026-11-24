package session

import "fanamix/domain/mix"

// AudibleTake is a take the monitor bus should play, with its resolved gain
// and the track effects heard on playback and export.
type AudibleTake struct {
	Take    Take
	Effects mix.Effects
}

// AudibleTakes resolves mute, solo and active-take selection into the set of
// takes the playback bus should mix.
func (s *Session) AudibleTakes() []AudibleTake {
	soloed := s.HasSolo()
	var audible []AudibleTake
	for _, track := range s.tracks {
		if !track.IsAudible(soloed) || track.activeTakeID.IsZero() {
			continue
		}
		for _, take := range s.takes {
			if take.trackID != track.id || take.muted || take.AudibleLength() == 0 {
				continue
			}
			if take.id != track.activeTakeID && !take.layered {
				continue
			}
			audible = append(audible, AudibleTake{Take: take, Effects: track.effects})
		}
	}
	return audible
}

// NextTakeNumber returns the sequence number the next take file should use.
func (s *Session) NextTakeNumber() int { return s.nextTake }
