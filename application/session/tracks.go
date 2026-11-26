package sessionapp

import (
	"errors"

	"fanamix/domain/mix"
	"fanamix/domain/session"
	"fanamix/domain/studio"
)

// AddTrack appends a track bound to one device channel.
func (s *Service) AddTrack(name string, inputChannel int) (session.Track, error) {
	return s.AddTrackWithSource(name, session.InputSource(inputChannel))
}

// AddTrackWithSource appends a track for any supported source.
func (s *Service) AddTrackWithSource(name string, source session.Source) (session.Track, error) {
	if err := s.requireSession(); err != nil {
		return session.Track{}, err
	}
	track, err := s.current.AddTrackWithSource(name, source)
	if err != nil {
		return session.Track{}, err
	}
	return track, s.Save()
}

// RenameTrack changes a track name.
func (s *Service) RenameTrack(id session.TrackID, name string) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.RenameTrack(id, name); err != nil {
		return err
	}
	return s.Save()
}

// SetRepeat loops an instrument track's pattern the given number of times.
func (s *Service) SetRepeat(id session.TrackID, repeat int) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetRepeat(id, repeat); err != nil {
		return err
	}
	return s.Save()
}

// SetPatternFrames widens an instrument pattern past its last note.
func (s *Service) SetPatternFrames(id session.TrackID, frames int) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetPatternFrames(id, frames); err != nil {
		return err
	}
	return s.Save()
}

// SetArm arms or disarms a track and refuses conflicting sources.
func (s *Service) SetArm(id session.TrackID, armed bool) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetArm(id, armed); err != nil {
		return err
	}
	return s.Save()
}

// SetMute mutes or unmutes a track.
func (s *Service) SetMute(id session.TrackID, muted bool) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetMute(id, muted); err != nil {
		return err
	}
	return s.Save()
}

// SetSolo solos or unsolos a track.
func (s *Service) SetSolo(id session.TrackID, soloed bool) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetSolo(id, soloed); err != nil {
		return err
	}
	return s.Save()
}

// SetTrackVolume sets a track's monitor fader in decibels.
func (s *Service) SetTrackVolume(id session.TrackID, volumeDB float64) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetVolume(id, volumeDB); err != nil {
		return err
	}
	return s.Save()
}

// SetTrackPan places a track in the stereo field from -1 to 1.
func (s *Service) SetTrackPan(id session.TrackID, pan float64) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetPan(id, pan); err != nil {
		return err
	}
	return s.Save()
}

// SetSource changes where a track records from.
func (s *Service) SetSource(id session.TrackID, source session.Source) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetSource(id, source); err != nil {
		return err
	}
	return s.Save()
}

// SetInputChannel rebinds a track to another input channel.
func (s *Service) SetInputChannel(id session.TrackID, channel int) error {
	return s.SetSource(id, session.InputSource(channel))
}

// SetPrintEffects toggles baking the chain into the recorded file.
func (s *Service) SetPrintEffects(id session.TrackID, print bool) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetPrintEffects(id, print); err != nil {
		return err
	}
	return s.Save()
}

// SetEffects stores a track's effect chain.
func (s *Service) SetEffects(id session.TrackID, effects mix.Effects) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetEffects(id, effects); err != nil {
		return err
	}
	return s.Save()
}

// AddEffect appends an effect of the given kind to a track's chain.
func (s *Service) AddEffect(id session.TrackID, kind mix.EffectKind) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.AddEffect(id, kind); err != nil {
		return err
	}
	return s.Save()
}

// RemoveEffect drops one position from a track's chain.
func (s *Service) RemoveEffect(id session.TrackID, index int) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.RemoveEffect(id, index); err != nil {
		return err
	}
	return s.Save()
}

// MoveEffect reorders a track's chain.
func (s *Service) MoveEffect(id session.TrackID, from, to int) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.MoveEffect(id, from, to); err != nil {
		return err
	}
	return s.Save()
}

// RemoveTrack deletes a track and moves its takes to the session trash.
func (s *Service) RemoveTrack(id session.TrackID) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	orphaned, err := s.current.RemoveTrack(id)
	if err != nil {
		return err
	}
	for _, take := range orphaned {
		if s.current.TakeFileInUse(take.File()) {
			continue
		}
		_ = s.takes.MoveToTrash(s.root, take.File())
	}
	return s.Save()
}

// SetClick stores the click configuration.
func (s *Service) SetClick(click mix.Click) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	s.current.SetClick(click)
	return s.Save()
}

// SetLatencyOffset stores the record latency compensation in frames.
func (s *Service) SetLatencyOffset(frames int) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetLatencyOffset(frames); err != nil {
		return err
	}
	return s.Save()
}

// Chains maps every interface track to the engine's chain settings.
func (s *Service) Chains() []studio.ChainSettings {
	if s.current == nil {
		return nil
	}
	tracks := s.current.Tracks()
	chains := make([]studio.ChainSettings, 0, len(tracks))
	for _, track := range tracks {
		if track.IsSystemAudio() || track.IsInstrument() {
			continue
		}
		chains = append(chains, studio.ChainSettings{
			Channel:      track.InputChannel(),
			Effects:      track.Effects(),
			PrintEffects: track.PrintsEffects(),
		})
	}
	return chains
}

func (s *Service) requireSession() error {
	if s.current == nil {
		return errors.New("no session is open")
	}
	return nil
}
