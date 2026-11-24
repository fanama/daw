package sessionapp

import (
	"errors"

	"fanamix/domain/session"
	"fanamix/domain/studio"
)

// BeginTake creates the legacy single multichannel take entry before the
// engine rolls. Mixed PC audio and inputs need BeginTrackTakes instead.
func (s *Service) BeginTake(startFrame int) (session.Take, []session.ArmSlot, error) {
	if err := s.requireSession(); err != nil {
		return session.Take{}, nil, err
	}
	plan, err := s.current.ArmPlan()
	if err != nil {
		return session.Take{}, nil, err
	}
	if hasMixedSources(plan) {
		return session.Take{}, nil, errors.New("multi-source recording needs per-track takes")
	}
	number := s.current.NextTakeNumber()
	file := s.takes.TakePath(s.root, number)
	channels := len(plan)
	printed := false
	if plan[0].Source.IsSystem() {
		channels = 2
	} else {
		for _, slot := range plan {
			printed = printed || slot.PrintEffects
		}
	}
	take, err := s.current.AddTake(
		plan[0].TrackID,
		file,
		startFrame,
		channels,
		s.current.SampleRate(),
		printed,
		true,
		s.now(),
	)
	if err != nil {
		return session.Take{}, nil, err
	}
	return take, plan, s.Save()
}

// BeginTrackTakes creates one take per armed track, including a PC audio track
// captured alongside interface inputs. The desktop keeps its single
// multichannel take through BeginTake.
func (s *Service) BeginTrackTakes(startFrame int) ([]session.Take, []session.ArmSlot, error) {
	if err := s.requireSession(); err != nil {
		return nil, nil, err
	}
	plan, err := s.current.ArmPlan()
	if err != nil {
		return nil, nil, err
	}
	takes := make([]session.Take, 0, len(plan))
	for _, slot := range plan {
		channels := 1
		if slot.Source.IsSystem() {
			channels = 2
		}
		file := s.takes.TakePath(s.root, s.current.NextTakeNumber())
		take, addErr := s.current.AddTake(
			slot.TrackID,
			file,
			startFrame,
			channels,
			s.current.SampleRate(),
			slot.PrintEffects,
			true,
			s.now(),
		)
		if addErr != nil {
			return nil, nil, addErr
		}
		takes = append(takes, take)
	}
	if err := s.Save(); err != nil {
		return nil, nil, err
	}
	return takes, plan, nil
}

func hasMixedSources(plan []session.ArmSlot) bool {
	hasInput, hasSystem := false, false
	for _, slot := range plan {
		if slot.Source.IsSystem() {
			hasSystem = true
		} else {
			hasInput = true
		}
	}
	return hasInput && hasSystem
}

// FinishTake stores the recorded length and whether the take was interrupted.
func (s *Service) FinishTake(id session.TakeID, frames int, incomplete bool) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.FinishTake(id, frames, incomplete); err != nil {
		return err
	}
	return s.Save()
}

// AbortTake removes a take entry whose file never opened.
func (s *Service) AbortTake(id session.TakeID) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if _, err := s.current.RemoveTake(id); err != nil {
		return err
	}
	return s.Save()
}

// SetActiveTake chooses the audible take of a track.
func (s *Service) SetActiveTake(trackID session.TrackID, takeID session.TakeID) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetActiveTake(trackID, takeID); err != nil {
		return err
	}
	return s.Save()
}

// NormalizeTake sets a take gain so its loudest peak lands at the session
// normalize target. The recorded file is never rewritten.
func (s *Service) NormalizeTake(id session.TakeID) (float64, error) {
	if err := s.requireSession(); err != nil {
		return 0, err
	}
	take, ok := s.current.FindTake(id)
	if !ok {
		return 0, errors.New("take does not exist")
	}
	audio, err := s.takes.OpenTake(s.root, take.File())
	if err != nil {
		return 0, err
	}
	peak, err := scanPeak(audio)
	_ = audio.Close()
	if err != nil {
		return 0, err
	}
	gain, ok := session.NormalizeGainDB(peak, session.NormalizeTargetDB)
	if !ok {
		return 0, errors.New("take is silent; nothing to normalize")
	}
	if err := s.current.SetTakeGain(id, gain); err != nil {
		return 0, err
	}
	return gain, s.Save()
}

func scanPeak(audio session.TakeAudio) (float32, error) {
	channels := audio.Channels()
	if channels < 1 {
		return 0, errors.New("take has no channels")
	}
	buffer := make([]float32, 8192*channels)
	peak := float32(0)
	for frame := 0; frame < audio.Frames(); {
		read, err := audio.ReadFrames(frame, buffer)
		if err != nil {
			return 0, err
		}
		if read == 0 {
			break
		}
		for index := 0; index < read*channels; index++ {
			value := buffer[index]
			if value < 0 {
				value = -value
			}
			if value > peak {
				peak = value
			}
		}
		frame += read
	}
	return peak, nil
}

// SetTakeGain sets a take's playback gain.
func (s *Service) SetTakeGain(id session.TakeID, gainDB float64) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetTakeGain(id, gainDB); err != nil {
		return err
	}
	return s.Save()
}

// SetTakeTrim sets non-destructive in and out points.
func (s *Service) SetTakeTrim(id session.TakeID, trim session.Trim) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetTakeTrim(id, trim); err != nil {
		return err
	}
	return s.Save()
}

// SetTakeMute mutes one take inside a stack.
func (s *Service) SetTakeMute(id session.TakeID, muted bool) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	if err := s.current.SetTakeMute(id, muted); err != nil {
		return err
	}
	return s.Save()
}

// DeleteTake removes a take and moves its audio to the trash.
func (s *Service) DeleteTake(id session.TakeID) error {
	if err := s.requireSession(); err != nil {
		return err
	}
	removed, err := s.current.RemoveTake(id)
	if err != nil {
		return err
	}
	if s.current.TakeFileInUse(removed.File()) {
		return s.Save()
	}
	if err := s.takes.MoveToTrash(s.root, removed.File()); err != nil {
		return err
	}
	return s.Save()
}

// AudibleTakes resolves the playback mix selection.
func (s *Service) AudibleTakes() []session.AudibleTake {
	if s.current == nil {
		return nil
	}
	return s.current.AudibleTakes()
}

// TakeFrameLimit reports the capacity ceiling for the armed channel count.
func (s *Service) TakeFrameLimit() int {
	if s.current == nil {
		return 0
	}
	return session.TakeFrameLimit(s.current.ArmedChannelCount())
}

// InputChains maps interface tracks to engine chain settings.
func (s *Service) InputChains() []studio.ChainSettings {
	return s.Chains()
}
