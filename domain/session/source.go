package session

// SourceKind distinguishes where a track's audio comes from.
type SourceKind string

const (
	// SourceInput is one channel of the audio interface.
	SourceInput SourceKind = "input"
	// SourceSystem is the computer's own output (loopback or virtual device).
	SourceSystem SourceKind = "system"
	// SourceInstrument is a built-in software instrument played from notes.
	SourceInstrument SourceKind = "instrument"
)

// Source is the origin of a track: an interface input channel, the computer's
// own playback audio, or a built-in instrument.
type Source struct {
	Kind       SourceKind
	Channel    int
	Instrument Instrument
}

// InputSource returns the source for one interface channel.
func InputSource(channel int) Source {
	return Source{Kind: SourceInput, Channel: channel}
}

// SystemSource returns the source for the computer's own output.
func SystemSource() Source {
	return Source{Kind: SourceSystem}
}

// InstrumentSource returns the source for a built-in software instrument.
func InstrumentSource(instrument Instrument) Source {
	return Source{Kind: SourceInstrument, Instrument: instrument}
}

// IsSystem reports whether the source is the computer output.
func (s Source) IsSystem() bool { return s.Kind == SourceSystem }

// IsInput reports whether the source is an interface channel.
func (s Source) IsInput() bool { return s.Kind == SourceInput || s.Kind == "" }

// IsInstrument reports whether the source is a software instrument.
func (s Source) IsInstrument() bool { return s.Kind == SourceInstrument }

// Label is the human-readable description used in the window.
func (s Source) Label() string {
	if s.IsSystem() {
		return "PC audio"
	}
	if s.IsInstrument() {
		return s.Instrument.String()
	}
	return "input " + itoa(s.Channel)
}

func (s Source) normalized() (Source, error) {
	switch {
	case s.IsInput():
		if err := validateInputChannel(s.Channel); err != nil {
			return Source{}, err
		}
		s.Kind = SourceInput
		return s, nil
	case s.IsSystem():
		s.Channel = 0
		s.Instrument = ""
		return s, nil
	case s.IsInstrument():
		instrument, err := s.Instrument.normalized()
		if err != nil {
			return Source{}, err
		}
		s.Channel = 0
		s.Instrument = instrument
		return s, nil
	default:
		return Source{}, invalid("unknown track source %q", s.Kind)
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	negative := value < 0
	if negative {
		value = -value
	}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
