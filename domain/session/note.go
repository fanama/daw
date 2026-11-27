package session

const (
	// MinNotePitch is the lowest MIDI note number.
	MinNotePitch = 0
	// MaxNotePitch is the highest MIDI note number.
	MaxNotePitch = 127
	// DefaultNoteVelocity is used when a caller omits the velocity.
	DefaultNoteVelocity = 100
	// MaxNoteVelocity is the loudest MIDI velocity.
	MaxNoteVelocity = 127
)

// Note is one timed MIDI note on an instrument track. Its positions use the
// session frame rate, so notes stay aligned with recorded takes.
type Note struct {
	id           NoteID
	trackID      TrackID
	pitch        int
	startFrame   int
	lengthFrames int
	velocity     int
}

// NoteSpec is a note to add: its pitch and timing, before it has an identity.
type NoteSpec struct {
	Pitch        int
	StartFrame   int
	LengthFrames int
	Velocity     int
}

func newNote(id NoteID, trackID TrackID, pitch, startFrame, lengthFrames, velocity int) (Note, error) {
	if pitch < MinNotePitch || pitch > MaxNotePitch {
		return Note{}, invalid("note pitch %d is outside 0..127", pitch)
	}
	if startFrame < 0 {
		return Note{}, invalid("note start must not be negative")
	}
	if lengthFrames < 1 {
		return Note{}, invalid("note length must be positive")
	}
	if velocity <= 0 {
		velocity = DefaultNoteVelocity
	}
	if velocity > MaxNoteVelocity {
		velocity = MaxNoteVelocity
	}
	return Note{
		id:           id,
		trackID:      trackID,
		pitch:        pitch,
		startFrame:   startFrame,
		lengthFrames: lengthFrames,
		velocity:     velocity,
	}, nil
}

func noteFromState(state NoteState) (Note, error) {
	return newNote(state.ID, state.TrackID, state.Pitch, state.StartFrame, state.LengthFrames, state.Velocity)
}

// ID returns the note identifier.
func (n Note) ID() NoteID { return n.id }

// TrackID returns the owning instrument track.
func (n Note) TrackID() TrackID { return n.trackID }

// Pitch is the MIDI note number, 0..127.
func (n Note) Pitch() int { return n.pitch }

// StartFrame is the first frame the note sounds on.
func (n Note) StartFrame() int { return n.startFrame }

// LengthFrames is how long the note is held.
func (n Note) LengthFrames() int { return n.lengthFrames }

// EndFrame is the first frame after the note.
func (n Note) EndFrame() int { return n.startFrame + n.lengthFrames }

// Velocity is the MIDI velocity, 1..127.
func (n Note) Velocity() int { return n.velocity }

func (n Note) state() NoteState {
	return NoteState{
		ID:           n.id,
		TrackID:      n.trackID,
		Pitch:        n.pitch,
		StartFrame:   n.startFrame,
		LengthFrames: n.lengthFrames,
		Velocity:     n.velocity,
	}
}
