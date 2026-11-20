package session

import "time"

// Trim is a non-destructive in/out point measured from the start of the take.
type Trim struct {
	StartFrame int
	EndFrame   int
}

// AudioOffset returns where playback starts inside the file.
func (t Trim) AudioOffset() int {
	if t.StartFrame < 0 {
		return 0
	}
	return t.StartFrame
}

// Take is one recorded pass on one track.
type Take struct {
	id           TakeID
	trackID      TrackID
	file         string
	startFrame   int
	lengthFrames int
	channels     int
	sampleRate   int
	printed      bool
	incomplete   bool
	muted        bool
	trim         Trim
	gainDB       float64
	createdAt    time.Time
}

// ID returns the take identifier.
func (t Take) ID() TakeID { return t.id }

// TrackID returns the owning track.
func (t Take) TrackID() TrackID { return t.trackID }

// File returns the session-relative audio path.
func (t Take) File() string { return t.file }

// StartFrame returns the position relative to session zero.
func (t Take) StartFrame() int { return t.startFrame }

// LengthFrames returns the recorded length.
func (t Take) LengthFrames() int { return t.lengthFrames }

// EndFrame returns the position where the take stops playing.
func (t Take) EndFrame() int { return t.startFrame + t.AudibleLength() }

// Channels returns the recorded channel count.
func (t Take) Channels() int { return t.channels }

// SampleRate returns the recorded frame rate.
func (t Take) SampleRate() int { return t.sampleRate }

// IsPrinted reports whether effects were baked into the file.
func (t Take) IsPrinted() bool { return t.printed }

// IsIncomplete reports whether the take survived an abnormal stop.
func (t Take) IsIncomplete() bool { return t.incomplete }

// IsMuted reports whether this individual take is muted.
func (t Take) IsMuted() bool { return t.muted }

// Trim returns the non-destructive in/out points.
func (t Take) Trim() Trim { return t.trim }

// GainDB returns the take gain.
func (t Take) GainDB() float64 { return t.gainDB }

// CreatedAt returns the recording timestamp.
func (t Take) CreatedAt() time.Time { return t.createdAt }

// AudibleLength returns the trimmed duration in frames.
func (t Take) AudibleLength() int {
	trimmed := t.lengthFrames - t.trim.StartFrame
	if t.trim.EndFrame > 0 {
		trimmed = t.trim.EndFrame - t.trim.StartFrame
	}
	if trimmed < 0 {
		return 0
	}
	return trimmed
}

func (t Take) state() TakeState {
	return TakeState{
		ID:           t.id,
		TrackID:      t.trackID,
		File:         t.file,
		StartFrame:   t.startFrame,
		LengthFrames: t.lengthFrames,
		Channels:     t.channels,
		SampleRate:   t.sampleRate,
		Printed:      t.printed,
		Incomplete:   t.incomplete,
		Muted:        t.muted,
		Trim:         t.trim,
		GainDB:       t.gainDB,
		CreatedAt:    t.createdAt,
	}
}

func takeFromState(state TakeState) (Take, error) {
	if state.File == "" {
		return Take{}, invalid("take %s has no audio file", state.ID)
	}
	if state.StartFrame < 0 {
		return Take{}, invalid("take %s has a negative start frame", state.ID)
	}
	if state.LengthFrames < 0 {
		return Take{}, invalid("take %s has a negative length", state.ID)
	}
	if state.Channels <= 0 {
		return Take{}, invalid("take %s has %d channels", state.ID, state.Channels)
	}
	if state.SampleRate < minSampleRate || state.SampleRate > maxSampleRate {
		return Take{}, invalid("take %s has unsupported sample rate %d", state.ID, state.SampleRate)
	}
	take := Take{
		id:           state.ID,
		trackID:      state.TrackID,
		file:         state.File,
		startFrame:   state.StartFrame,
		lengthFrames: state.LengthFrames,
		channels:     state.Channels,
		sampleRate:   state.SampleRate,
		printed:      state.Printed,
		incomplete:   state.Incomplete,
		muted:        state.Muted,
		trim:         state.Trim,
		gainDB:       state.GainDB,
		createdAt:    state.CreatedAt.UTC(),
	}
	if take.trim.StartFrame < 0 || take.trim.StartFrame > take.lengthFrames {
		return Take{}, invalid("take %s trim start is outside the take", state.ID)
	}
	if take.trim.EndFrame > take.lengthFrames {
		return Take{}, invalid("take %s trim end is outside the take", state.ID)
	}
	return take, nil
}
