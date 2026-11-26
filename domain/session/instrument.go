package session

// Instrument is a built-in software sound for a note-based track.
type Instrument string

const (
	// InstrumentPiano is an acoustic piano voice.
	InstrumentPiano Instrument = "piano"
	// InstrumentBass is an electric bass voice.
	InstrumentBass Instrument = "bass"
	// InstrumentGuitar is a plucked electric guitar voice.
	InstrumentGuitar Instrument = "guitar"
	// InstrumentDrums is a drum kit played from a step sequencer.
	InstrumentDrums Instrument = "drums"
)

// String renders the instrument for display and persistence.
func (i Instrument) String() string { return string(i) }

func (i Instrument) normalized() (Instrument, error) {
	switch i {
	case InstrumentPiano, InstrumentBass, InstrumentGuitar, InstrumentDrums:
		return i, nil
	default:
		return "", invalid("unknown instrument %q", i)
	}
}
