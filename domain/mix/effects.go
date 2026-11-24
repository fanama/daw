// Package mix defines the effects vocabulary shared by sessions, the audio
// engine and the DSP adapters. It is a shared kernel: value objects and ports
// only, no delivery, persistence or vendor concepts.
package mix

// EQBand is one parametric band of the track equalizer.
type EQBand struct {
	FrequencyHz float64
	Q           float64
	GainDB      float64
}

// EQ is a five band parametric equalizer spanning 20 Hz to 20 kHz.
type EQ struct {
	Enabled bool
	Bands   [5]EQBand
}

// Amp is the guitar front end: drive into a bass/mid/treble tone stack.
type Amp struct {
	Enabled  bool
	Drive    float64
	BassDB   float64
	MidDB    float64
	TrebleDB float64
	LevelDB  float64
}

// Cab is the fixed speaker-emulation filter. Character is its single control.
type Cab struct {
	Enabled   bool
	Character float64
}

// Compressor is a feed-forward soft-knee dynamics processor.
type Compressor struct {
	Enabled     bool
	ThresholdDB float64
	Ratio       float64
	AttackMs    float64
	ReleaseMs   float64
	KneeDB      float64
	MakeupDB    float64
}

// Reverb is an algorithmic room reverb.
type Reverb struct {
	Enabled    bool
	Size       float64
	Damp       float64
	Mix        float64
	PreDelayMs float64
}

// Effects is the complete per-track chain in its fixed processing order:
// Amp -> Cab -> EQ -> Compressor -> Reverb.
type Effects struct {
	Amp    Amp
	Cab    Cab
	EQ     EQ
	Comp   Compressor
	Reverb Reverb
}

// DefaultEffects returns the chain a new guitar track starts with: a mild amp
// and cabinet, a flat equalizer, and dynamics and reverb off.
func DefaultEffects() Effects {
	return Effects{
		Amp: Amp{Enabled: true, Drive: 0.35, LevelDB: -6},
		Cab: Cab{Enabled: true, Character: 0.5},
		EQ: EQ{
			Enabled: true,
			Bands: [5]EQBand{
				{FrequencyHz: 20, Q: 0.7},
				{FrequencyHz: 100, Q: 0.7},
				{FrequencyHz: 1000, Q: 0.9},
				{FrequencyHz: 6000, Q: 0.7},
				{FrequencyHz: 20000, Q: 0.7},
			},
		},
		Comp:   Compressor{Enabled: false, ThresholdDB: -18, Ratio: 3, AttackMs: 10, ReleaseMs: 120, KneeDB: 6},
		Reverb: Reverb{Enabled: false, Size: 0.5, Damp: 0.5, Mix: 0.2, PreDelayMs: 10},
	}
}

// DryEffects returns a chain with every block disabled. PC audio tracks start
// dry so a shared tab is played back as-is until a block is enabled.
func DryEffects() Effects {
	effects := DefaultEffects()
	effects.Amp.Enabled = false
	effects.Cab.Enabled = false
	effects.EQ.Enabled = false
	effects.Comp.Enabled = false
	effects.Reverb.Enabled = false
	return effects
}

// Bypassed reports whether no block processes the signal.
func (e Effects) Bypassed() bool {
	return !e.Amp.Enabled && !e.Cab.Enabled && !e.EQ.Enabled && !e.Comp.Enabled && !e.Reverb.Enabled
}

// Normalize returns a clamped copy so the aggregate never stores values the
// DSP cannot honor.
func (e Effects) Normalize() Effects {
	normalized := e
	normalized.Amp = e.Amp.normalize()
	normalized.Cab = e.Cab.normalize()
	normalized.EQ = e.EQ.normalize()
	normalized.Comp = e.Comp.normalize()
	normalized.Reverb = e.Reverb.normalize()
	return normalized
}

func (a Amp) normalize() Amp {
	return Amp{
		Enabled:  a.Enabled,
		Drive:    clamp(a.Drive, 0, 1),
		BassDB:   clamp(a.BassDB, -12, 12),
		MidDB:    clamp(a.MidDB, -12, 12),
		TrebleDB: clamp(a.TrebleDB, -12, 12),
		LevelDB:  clamp(a.LevelDB, -24, 6),
	}
}

func (c Cab) normalize() Cab {
	return Cab{Enabled: c.Enabled, Character: clamp(c.Character, 0, 1)}
}

func (e EQ) normalize() EQ {
	normalized := EQ{Enabled: e.Enabled}
	for i, band := range e.Bands {
		normalized.Bands[i] = EQBand{
			FrequencyHz: clamp(band.FrequencyHz, 20, 20000),
			Q:           clamp(band.Q, 0.3, 10),
			GainDB:      clamp(band.GainDB, -24, 24),
		}
	}
	return normalized
}

func (c Compressor) normalize() Compressor {
	return Compressor{
		Enabled:     c.Enabled,
		ThresholdDB: clamp(c.ThresholdDB, -60, 0),
		Ratio:       clamp(c.Ratio, 1, 20),
		AttackMs:    clamp(c.AttackMs, 0.1, 100),
		ReleaseMs:   clamp(c.ReleaseMs, 10, 1000),
		KneeDB:      clamp(c.KneeDB, 0, 12),
		MakeupDB:    clamp(c.MakeupDB, 0, 24),
	}
}

func (r Reverb) normalize() Reverb {
	return Reverb{
		Enabled:    r.Enabled,
		Size:       clamp(r.Size, 0, 1),
		Damp:       clamp(r.Damp, 0, 1),
		Mix:        clamp(r.Mix, 0, 1),
		PreDelayMs: clamp(r.PreDelayMs, 0, 100),
	}
}

func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
