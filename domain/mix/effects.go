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

// EffectKind identifies one processor a chain can hold.
type EffectKind string

const (
	EffectAmp    EffectKind = "amp"
	EffectCab    EffectKind = "cab"
	EffectEQ     EffectKind = "eq"
	EffectComp   EffectKind = "comp"
	EffectReverb EffectKind = "reverb"
)

// MaxEffectInstances bounds one track's chain so the DSP can keep its stages
// preallocated.
const MaxEffectInstances = 8

// Effect is one processor instance. Only the settings block matching Kind is
// populated, so the same kind can appear several times with its own settings.
type Effect struct {
	Kind   EffectKind
	Amp    *Amp
	Cab    *Cab
	EQ     *EQ
	Comp   *Compressor
	Reverb *Reverb
}

// Effects is a track's effect chain, processed in slice order.
type Effects struct {
	Chain []Effect
}

// ValidEffectKind reports whether kind names a processor the DSP can run.
func ValidEffectKind(kind EffectKind) bool {
	switch kind {
	case EffectAmp, EffectCab, EffectEQ, EffectComp, EffectReverb:
		return true
	}
	return false
}

// NewEffect returns an enabled instance of kind with the default settings.
func NewEffect(kind EffectKind) Effect {
	effect := Effect{Kind: kind}
	switch kind {
	case EffectAmp:
		effect.Amp = &Amp{Enabled: true, Drive: 0.35, LevelDB: -6}
	case EffectCab:
		effect.Cab = &Cab{Enabled: true, Character: 0.5}
	case EffectEQ:
		bands := defaultBands()
		effect.EQ = &EQ{Enabled: true, Bands: bands}
	case EffectComp:
		effect.Comp = &Compressor{Enabled: true, ThresholdDB: -18, Ratio: 3, AttackMs: 10, ReleaseMs: 120, KneeDB: 6}
	case EffectReverb:
		effect.Reverb = &Reverb{Enabled: true, Size: 0.5, Damp: 0.5, Mix: 0.2, PreDelayMs: 10}
	}
	return effect
}

// DefaultEffects returns the chain a new guitar track starts with: a mild amp
// and cabinet, a flat equalizer, and dynamics and reverb off.
func DefaultEffects() Effects {
	effects := Effects{Chain: []Effect{
		NewEffect(EffectAmp),
		NewEffect(EffectCab),
		NewEffect(EffectEQ),
		NewEffect(EffectComp),
		NewEffect(EffectReverb),
	}}
	effects.Chain[3].SetEnabled(false)
	effects.Chain[4].SetEnabled(false)
	return effects
}

// DryEffects returns the default chain with every instance switched off. PC
// audio tracks start dry so a shared tab is played back as-is.
func DryEffects() Effects {
	effects := DefaultEffects()
	for index := range effects.Chain {
		effects.Chain[index].SetEnabled(false)
	}
	return effects
}

// Bypassed reports whether no instance processes the signal.
func (e Effects) Bypassed() bool {
	for _, effect := range e.Chain {
		if effect.IsEnabled() {
			return false
		}
	}
	return true
}

// Find returns the first instance of kind, if the chain holds one.
func (e Effects) Find(kind EffectKind) (Effect, bool) {
	for _, effect := range e.Chain {
		if effect.Kind == kind {
			return effect, true
		}
	}
	return Effect{}, false
}

// Normalize returns a clamped copy of the chain: unknown kinds are dropped, the
// length is capped and every parameter is bounded to what the DSP honors.
func (e Effects) Normalize() Effects {
	chain := make([]Effect, 0, len(e.Chain))
	for _, effect := range e.Chain {
		if len(chain) >= MaxEffectInstances {
			break
		}
		if !ValidEffectKind(effect.Kind) {
			continue
		}
		chain = append(chain, effect.normalize())
	}
	return Effects{Chain: chain}
}

// IsEnabled reports whether this instance processes the signal.
func (e Effect) IsEnabled() bool {
	switch e.Kind {
	case EffectAmp:
		return e.Amp != nil && e.Amp.Enabled
	case EffectCab:
		return e.Cab != nil && e.Cab.Enabled
	case EffectEQ:
		return e.EQ != nil && e.EQ.Enabled
	case EffectComp:
		return e.Comp != nil && e.Comp.Enabled
	case EffectReverb:
		return e.Reverb != nil && e.Reverb.Enabled
	}
	return false
}

// SetEnabled switches one instance on or off.
func (e *Effect) SetEnabled(enabled bool) {
	switch e.Kind {
	case EffectAmp:
		if e.Amp != nil {
			e.Amp.Enabled = enabled
		}
	case EffectCab:
		if e.Cab != nil {
			e.Cab.Enabled = enabled
		}
	case EffectEQ:
		if e.EQ != nil {
			e.EQ.Enabled = enabled
		}
	case EffectComp:
		if e.Comp != nil {
			e.Comp.Enabled = enabled
		}
	case EffectReverb:
		if e.Reverb != nil {
			e.Reverb.Enabled = enabled
		}
	}
}

func (e Effect) normalize() Effect {
	normalized := Effect{Kind: e.Kind}
	switch e.Kind {
	case EffectAmp:
		amp := defaultAmp()
		if e.Amp != nil {
			amp = e.Amp.normalize()
		}
		normalized.Amp = &amp
	case EffectCab:
		cab := defaultCab()
		if e.Cab != nil {
			cab = e.Cab.normalize()
		}
		normalized.Cab = &cab
	case EffectEQ:
		eq := defaultEQ()
		if e.EQ != nil {
			eq = e.EQ.normalize()
		}
		normalized.EQ = &eq
	case EffectComp:
		comp := defaultCompressor()
		if e.Comp != nil {
			comp = e.Comp.normalize()
		}
		normalized.Comp = &comp
	case EffectReverb:
		reverb := defaultReverb()
		if e.Reverb != nil {
			reverb = e.Reverb.normalize()
		}
		normalized.Reverb = &reverb
	}
	return normalized
}

func defaultBands() [5]EQBand {
	return [5]EQBand{
		{FrequencyHz: 20, Q: 0.7},
		{FrequencyHz: 100, Q: 0.7},
		{FrequencyHz: 1000, Q: 0.9},
		{FrequencyHz: 6000, Q: 0.7},
		{FrequencyHz: 20000, Q: 0.7},
	}
}

func defaultAmp() Amp {
	return Amp{Enabled: true, Drive: 0.35, LevelDB: -6}
}

func defaultCab() Cab {
	return Cab{Enabled: true, Character: 0.5}
}

func defaultEQ() EQ {
	return EQ{Enabled: true, Bands: defaultBands()}
}

func defaultCompressor() Compressor {
	return Compressor{Enabled: false, ThresholdDB: -18, Ratio: 3, AttackMs: 10, ReleaseMs: 120, KneeDB: 6}
}

func defaultReverb() Reverb {
	return Reverb{Enabled: false, Size: 0.5, Damp: 0.5, Mix: 0.2, PreDelayMs: 10}
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
	defaults := defaultBands()
	normalized := EQ{Enabled: e.Enabled}
	for i, band := range e.Bands {
		frequency := band.FrequencyHz
		if frequency <= 0 {
			frequency = defaults[i].FrequencyHz
		}
		q := band.Q
		if q <= 0 {
			q = defaults[i].Q
		}
		normalized.Bands[i] = EQBand{
			FrequencyHz: clamp(frequency, 20, 20000),
			Q:           clamp(q, 0.3, 10),
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
