package session

import (
	"strings"
	"time"

	"gorecord/domain/mix"
)

const (
	minSampleRate   = 8000
	maxSampleRate   = 384000
	maxInputChannel = 63
)

// Session is the aggregate root for one song idea: its sample rate, tracks,
// takes and click settings. Every mutation goes through this type so the
// invariants hold.
type Session struct {
	name          string
	createdAt     time.Time
	sampleRate    int
	tracks        []Track
	takes         []Take
	click         mix.Click
	latencyOffset int
	nextTrack     int
	nextTake      int
}

// New creates an empty session at the given sample rate.
func New(name string, sampleRate int, now time.Time) (*Session, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, invalid("session name is required")
	}
	if sampleRate < minSampleRate || sampleRate > maxSampleRate {
		return nil, invalid("sample rate %d is outside %d..%d", sampleRate, minSampleRate, maxSampleRate)
	}
	return &Session{
		name:       cleanName,
		createdAt:  now.UTC(),
		sampleRate: sampleRate,
		click:      mix.DefaultClick(),
		nextTrack:  1,
		nextTake:   1,
	}, nil
}

// Name returns the session name.
func (s *Session) Name() string { return s.name }

// CreatedAt returns the creation timestamp.
func (s *Session) CreatedAt() time.Time { return s.createdAt }

// SampleRate returns the session frame rate.
func (s *Session) SampleRate() int { return s.sampleRate }

// Click returns the click configuration.
func (s *Session) Click() mix.Click { return s.click }

// SetClick stores a clamped click configuration.
func (s *Session) SetClick(click mix.Click) {
	s.click = click.Normalize()
}

// LatencyOffset returns the record latency compensation in frames.
func (s *Session) LatencyOffset() int { return s.latencyOffset }

// SetLatencyOffset stores a compensation offset bounded to one second.
func (s *Session) SetLatencyOffset(frames int) error {
	if frames > s.sampleRate || frames < -s.sampleRate {
		return invalid("latency offset must stay within one second")
	}
	s.latencyOffset = frames
	return nil
}

// Tracks returns a copy of the track collection.
func (s *Session) Tracks() []Track {
	tracks := make([]Track, len(s.tracks))
	copy(tracks, s.tracks)
	return tracks
}

// Takes returns a copy of the take collection.
func (s *Session) Takes() []Take {
	takes := make([]Take, len(s.takes))
	copy(takes, s.takes)
	return takes
}

// FindTrack returns one track by identifier.
func (s *Session) FindTrack(id TrackID) (Track, bool) {
	index, err := s.trackIndex(id)
	if err != nil {
		return Track{}, false
	}
	return s.tracks[index], true
}

// FindTake returns one take by identifier.
func (s *Session) FindTake(id TakeID) (Take, bool) {
	index, err := s.takeIndex(id)
	if err != nil {
		return Take{}, false
	}
	return s.takes[index], true
}

// TrackTakes returns the takes of one track in recording order.
func (s *Session) TrackTakes(id TrackID) []Take {
	var takes []Take
	for _, take := range s.takes {
		if take.trackID == id {
			takes = append(takes, take)
		}
	}
	return takes
}
