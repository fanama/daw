package session

// TrackID identifies a track inside one session.
type TrackID string

// TakeID identifies a recorded take inside one session.
type TakeID string

// MarkerID identifies a timeline marker inside one session.
type MarkerID string

// NoteID identifies a note inside one session.
type NoteID string

// String renders the identifier for display and logging.
func (id NoteID) String() string { return string(id) }

// IsZero reports whether the identifier is unset.
func (id NoteID) IsZero() bool { return id == "" }

// String renders the identifier for display and logging.
func (id TrackID) String() string { return string(id) }

// String renders the identifier for display and logging.
func (id TakeID) String() string { return string(id) }

// IsZero reports whether the identifier is unset.
func (id TrackID) IsZero() bool { return id == "" }

// IsZero reports whether the identifier is unset.
func (id TakeID) IsZero() bool { return id == "" }

// String renders the identifier for display and logging.
func (id MarkerID) String() string { return string(id) }

// IsZero reports whether the identifier is unset.
func (id MarkerID) IsZero() bool { return id == "" }
