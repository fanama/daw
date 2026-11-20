package session

import (
	"fmt"
	"math"
	"time"
)

// AddTake registers a recording that is starting or has just finished.
func (s *Session) AddTake(trackID TrackID, file string, startFrame, channels, sampleRate int, printed, incomplete bool, now time.Time) (Take, error) {
	if _, err := s.trackIndex(trackID); err != nil {
		return Take{}, err
	}
	if file == "" {
		return Take{}, invalid("take audio file is required")
	}
	if startFrame < 0 {
		return Take{}, invalid("take start frame cannot be negative")
	}
	if channels <= 0 {
		return Take{}, invalid("take must contain at least one channel")
	}
	if sampleRate != s.sampleRate {
		return Take{}, invalid("take sample rate %d does not match session rate %d", sampleRate, s.sampleRate)
	}
	take := Take{
		id:         TakeID(fmt.Sprintf("tk_%d", s.nextTake)),
		trackID:    trackID,
		file:       file,
		startFrame: startFrame,
		channels:   channels,
		sampleRate: sampleRate,
		printed:    printed,
		incomplete: incomplete,
		createdAt:  now.UTC(),
	}
	s.nextTake++
	s.takes = append(s.takes, take)
	s.activateIfEmpty(trackID, take.id)
	return take, nil
}

// CompleteTake records the final length of a finished take.
func (s *Session) CompleteTake(id TakeID, lengthFrames int) error {
	return s.FinishTake(id, lengthFrames, false)
}

// FinishTake records the final length and whether the take survived an
// abnormal stop, which never discards audio.
func (s *Session) FinishTake(id TakeID, lengthFrames int, incomplete bool) error {
	index, err := s.takeIndex(id)
	if err != nil {
		return err
	}
	if lengthFrames < 0 {
		return invalid("take length cannot be negative")
	}
	s.takes[index].lengthFrames = lengthFrames
	s.takes[index].incomplete = incomplete
	return nil
}

// SetActiveTake makes one take of a track the audible one.
func (s *Session) SetActiveTake(trackID TrackID, takeID TakeID) error {
	trackIndex, err := s.trackIndex(trackID)
	if err != nil {
		return err
	}
	takeIndex, err := s.takeIndex(takeID)
	if err != nil {
		return err
	}
	if s.takes[takeIndex].trackID != trackID {
		return invalid("take %s does not belong to track %s", takeID, trackID)
	}
	s.tracks[trackIndex].activeTakeID = takeID
	return nil
}

// SetTakeGain sets the non-destructive playback gain of a take.
func (s *Session) SetTakeGain(id TakeID, gainDB float64) error {
	index, err := s.takeIndex(id)
	if err != nil {
		return err
	}
	if math.IsNaN(gainDB) || math.IsInf(gainDB, 0) {
		return invalid("take gain must be a finite number")
	}
	s.takes[index].gainDB = math.Max(-24, math.Min(24, gainDB))
	return nil
}

// SetTakeTrim sets non-destructive in and out points.
func (s *Session) SetTakeTrim(id TakeID, trim Trim) error {
	index, err := s.takeIndex(id)
	if err != nil {
		return err
	}
	if trim.StartFrame < 0 || trim.EndFrame < 0 {
		return invalid("trim points cannot be negative")
	}
	if trim.EndFrame > 0 && trim.EndFrame <= trim.StartFrame {
		return invalid("trim end must come after trim start")
	}
	s.takes[index].trim = trim
	return nil
}

// SetTakeMute mutes or unmutes one take inside its stack.
func (s *Session) SetTakeMute(id TakeID, muted bool) error {
	index, err := s.takeIndex(id)
	if err != nil {
		return err
	}
	s.takes[index].muted = muted
	return nil
}

// RemoveTake deletes a take from the session and returns it so its audio can
// be moved to the trash.
func (s *Session) RemoveTake(id TakeID) (Take, error) {
	index, err := s.takeIndex(id)
	if err != nil {
		return Take{}, err
	}
	removed := s.takes[index]
	s.takes = append(s.takes[:index], s.takes[index+1:]...)
	s.reattachActiveTake(removed.trackID, id)
	return removed, nil
}

func (s *Session) activateIfEmpty(trackID TrackID, takeID TakeID) {
	index, err := s.trackIndex(trackID)
	if err != nil || !s.tracks[index].activeTakeID.IsZero() {
		return
	}
	s.tracks[index].activeTakeID = takeID
}

func (s *Session) reattachActiveTake(trackID TrackID, removed TakeID) {
	index, err := s.trackIndex(trackID)
	if err != nil || s.tracks[index].activeTakeID != removed {
		return
	}
	s.tracks[index].activeTakeID = ""
	for _, take := range s.takes {
		if take.trackID == trackID {
			s.tracks[index].activeTakeID = take.id
			return
		}
	}
}

func (s *Session) takeIndex(id TakeID) (int, error) {
	for index := range s.takes {
		if s.takes[index].id == id {
			return index, nil
		}
	}
	return -1, notFound("take %s does not exist", id)
}
