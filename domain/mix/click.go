package mix

// Click is the optional metronome. It is off by default, never recorded, and
// mixed only into the monitor output.
type Click struct {
	Enabled     bool
	BPM         float64
	BeatsPerBar int
	CountInBars int
}

// DefaultClick returns the click configuration for a new session.
func DefaultClick() Click {
	return Click{BPM: 120, BeatsPerBar: 4}
}

// Normalize clamps tempo and meter to the ranges the generator supports.
func (c Click) Normalize() Click {
	normalized := c
	if normalized.BPM < 20 {
		normalized.BPM = 20
	}
	if normalized.BPM > 300 {
		normalized.BPM = 300
	}
	if normalized.BeatsPerBar < 2 {
		normalized.BeatsPerBar = 2
	}
	if normalized.BeatsPerBar > 12 {
		normalized.BeatsPerBar = 12
	}
	if normalized.CountInBars < 0 {
		normalized.CountInBars = 0
	}
	if normalized.CountInBars > 2 {
		normalized.CountInBars = 2
	}
	return normalized
}

// FramesPerBeat converts tempo to frames at the given sample rate.
func (c Click) FramesPerBeat(sampleRate int) int {
	beatsPerSecond := c.BPM / 60
	if beatsPerSecond <= 0 || sampleRate <= 0 {
		return 0
	}
	return int(float64(sampleRate) / beatsPerSecond)
}
