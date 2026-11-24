package jsonstore

import "fanamix/domain/mix"

type bandJSON struct {
	FrequencyHz float64 `json:"frequencyHz"`
	Q           float64 `json:"q"`
	GainDB      float64 `json:"gainDb"`
}

type eqJSON struct {
	Enabled bool       `json:"enabled"`
	Bands   []bandJSON `json:"bands"`
}

type ampJSON struct {
	Enabled  bool    `json:"enabled"`
	Drive    float64 `json:"drive"`
	BassDB   float64 `json:"bassDb"`
	MidDB    float64 `json:"midDb"`
	TrebleDB float64 `json:"trebleDb"`
	LevelDB  float64 `json:"levelDb"`
}

type cabJSON struct {
	Enabled   bool    `json:"enabled"`
	Character float64 `json:"character"`
}

type compressorJSON struct {
	Enabled     bool    `json:"enabled"`
	ThresholdDB float64 `json:"thresholdDb"`
	Ratio       float64 `json:"ratio"`
	AttackMs    float64 `json:"attackMs"`
	ReleaseMs   float64 `json:"releaseMs"`
	KneeDB      float64 `json:"kneeDb"`
	MakeupDB    float64 `json:"makeupDb"`
}

type reverbJSON struct {
	Enabled    bool    `json:"enabled"`
	Size       float64 `json:"size"`
	Damp       float64 `json:"damp"`
	Mix        float64 `json:"mix"`
	PreDelayMs float64 `json:"preDelayMs"`
}

// effectJSON is one chain position. Only the block matching Kind is written.
type effectJSON struct {
	Kind   string          `json:"kind"`
	Amp    *ampJSON        `json:"amp,omitempty"`
	Cab    *cabJSON        `json:"cab,omitempty"`
	EQ     *eqJSON         `json:"eq,omitempty"`
	Comp   *compressorJSON `json:"comp,omitempty"`
	Reverb *reverbJSON     `json:"reverb,omitempty"`
}

// effectsJSON is a track's ordered chain. Projects written before schema 5 keep
// the five fixed blocks at the top level; those are still read and migrated.
type effectsJSON struct {
	Chain  []effectJSON    `json:"chain"`
	Amp    *ampJSON        `json:"amp,omitempty"`
	Cab    *cabJSON        `json:"cab,omitempty"`
	EQ     *eqJSON         `json:"eq,omitempty"`
	Comp   *compressorJSON `json:"comp,omitempty"`
	Reverb *reverbJSON     `json:"reverb,omitempty"`
}

func effectsFromDomain(effects mix.Effects) effectsJSON {
	stored := effectsJSON{Chain: make([]effectJSON, 0, len(effects.Chain))}
	for _, effect := range effects.Chain {
		entry := effectJSON{Kind: string(effect.Kind)}
		switch effect.Kind {
		case mix.EffectAmp:
			if effect.Amp != nil {
				amp := ampFromDomain(*effect.Amp)
				entry.Amp = &amp
			}
		case mix.EffectCab:
			if effect.Cab != nil {
				cab := cabFromDomain(*effect.Cab)
				entry.Cab = &cab
			}
		case mix.EffectEQ:
			if effect.EQ != nil {
				eq := eqFromDomain(*effect.EQ)
				entry.EQ = &eq
			}
		case mix.EffectComp:
			if effect.Comp != nil {
				comp := compressorFromDomain(*effect.Comp)
				entry.Comp = &comp
			}
		case mix.EffectReverb:
			if effect.Reverb != nil {
				reverb := reverbFromDomain(*effect.Reverb)
				entry.Reverb = &reverb
			}
		}
		stored.Chain = append(stored.Chain, entry)
	}
	return stored
}

// effectsToDomain restores the chain. A nil chain means a legacy project, whose
// five blocks are merged into the default chain; an explicit empty list means
// the track really has no effects.
func effectsToDomain(stored effectsJSON) mix.Effects {
	if stored.Chain != nil {
		chain := make([]mix.Effect, 0, len(stored.Chain))
		for _, entry := range stored.Chain {
			kind := mix.EffectKind(entry.Kind)
			if !mix.ValidEffectKind(kind) {
				continue
			}
			chain = append(chain, effectToDomain(kind, entry))
		}
		return mix.Effects{Chain: chain}.Normalize()
	}
	if !stored.hasLegacyBlocks() {
		return mix.DefaultEffects()
	}
	effects := mix.DefaultEffects()
	for index := range effects.Chain {
		effect := &effects.Chain[index]
		switch effect.Kind {
		case mix.EffectAmp:
			if stored.Amp != nil {
				amp := ampToDomain(*stored.Amp)
				effect.Amp = &amp
			}
		case mix.EffectCab:
			if stored.Cab != nil {
				cab := cabToDomain(*stored.Cab)
				effect.Cab = &cab
			}
		case mix.EffectEQ:
			if stored.EQ != nil {
				eq := eqToDomain(*stored.EQ)
				effect.EQ = &eq
			}
		case mix.EffectComp:
			if stored.Comp != nil {
				comp := compressorToDomain(*stored.Comp)
				effect.Comp = &comp
			}
		case mix.EffectReverb:
			if stored.Reverb != nil {
				reverb := reverbToDomain(*stored.Reverb)
				effect.Reverb = &reverb
			}
		}
	}
	return effects.Normalize()
}

func (e effectsJSON) hasLegacyBlocks() bool {
	return e.Amp != nil || e.Cab != nil || e.EQ != nil || e.Comp != nil || e.Reverb != nil
}

func effectToDomain(kind mix.EffectKind, entry effectJSON) mix.Effect {
	effect := mix.NewEffect(kind)
	switch kind {
	case mix.EffectAmp:
		if entry.Amp != nil {
			amp := ampToDomain(*entry.Amp)
			effect.Amp = &amp
		}
	case mix.EffectCab:
		if entry.Cab != nil {
			cab := cabToDomain(*entry.Cab)
			effect.Cab = &cab
		}
	case mix.EffectEQ:
		if entry.EQ != nil {
			eq := eqToDomain(*entry.EQ)
			effect.EQ = &eq
		}
	case mix.EffectComp:
		if entry.Comp != nil {
			comp := compressorToDomain(*entry.Comp)
			effect.Comp = &comp
		}
	case mix.EffectReverb:
		if entry.Reverb != nil {
			reverb := reverbToDomain(*entry.Reverb)
			effect.Reverb = &reverb
		}
	}
	return effect
}

func ampFromDomain(amp mix.Amp) ampJSON {
	return ampJSON{
		Enabled:  amp.Enabled,
		Drive:    amp.Drive,
		BassDB:   amp.BassDB,
		MidDB:    amp.MidDB,
		TrebleDB: amp.TrebleDB,
		LevelDB:  amp.LevelDB,
	}
}

func ampToDomain(stored ampJSON) mix.Amp {
	return mix.Amp{
		Enabled:  stored.Enabled,
		Drive:    stored.Drive,
		BassDB:   stored.BassDB,
		MidDB:    stored.MidDB,
		TrebleDB: stored.TrebleDB,
		LevelDB:  stored.LevelDB,
	}
}

func cabFromDomain(cab mix.Cab) cabJSON {
	return cabJSON{Enabled: cab.Enabled, Character: cab.Character}
}

func cabToDomain(stored cabJSON) mix.Cab {
	return mix.Cab{Enabled: stored.Enabled, Character: stored.Character}
}

func eqFromDomain(eq mix.EQ) eqJSON {
	bands := make([]bandJSON, 0, len(eq.Bands))
	for _, band := range eq.Bands {
		bands = append(bands, bandJSON{FrequencyHz: band.FrequencyHz, Q: band.Q, GainDB: band.GainDB})
	}
	return eqJSON{Enabled: eq.Enabled, Bands: bands}
}

// eqToDomain restores the equalizer and migrates the three bands of a v1
// project into the middle slots, leaving the 20 Hz and 20 kHz bands flat.
func eqToDomain(stored eqJSON) mix.EQ {
	eq := defaultEQ()
	eq.Enabled = stored.Enabled
	offset := 0
	if len(stored.Bands) == 3 && len(eq.Bands) == 5 {
		offset = 1
	}
	for index := 0; index < len(stored.Bands) && index+offset < len(eq.Bands); index++ {
		eq.Bands[index+offset] = mix.EQBand{
			FrequencyHz: stored.Bands[index].FrequencyHz,
			Q:           stored.Bands[index].Q,
			GainDB:      stored.Bands[index].GainDB,
		}
	}
	return eq
}

func compressorFromDomain(comp mix.Compressor) compressorJSON {
	return compressorJSON{
		Enabled:     comp.Enabled,
		ThresholdDB: comp.ThresholdDB,
		Ratio:       comp.Ratio,
		AttackMs:    comp.AttackMs,
		ReleaseMs:   comp.ReleaseMs,
		KneeDB:      comp.KneeDB,
		MakeupDB:    comp.MakeupDB,
	}
}

func compressorToDomain(stored compressorJSON) mix.Compressor {
	return mix.Compressor{
		Enabled:     stored.Enabled,
		ThresholdDB: stored.ThresholdDB,
		Ratio:       stored.Ratio,
		AttackMs:    stored.AttackMs,
		ReleaseMs:   stored.ReleaseMs,
		KneeDB:      stored.KneeDB,
		MakeupDB:    stored.MakeupDB,
	}
}

func reverbFromDomain(reverb mix.Reverb) reverbJSON {
	return reverbJSON{
		Enabled:    reverb.Enabled,
		Size:       reverb.Size,
		Damp:       reverb.Damp,
		Mix:        reverb.Mix,
		PreDelayMs: reverb.PreDelayMs,
	}
}

func reverbToDomain(stored reverbJSON) mix.Reverb {
	return mix.Reverb{
		Enabled:    stored.Enabled,
		Size:       stored.Size,
		Damp:       stored.Damp,
		Mix:        stored.Mix,
		PreDelayMs: stored.PreDelayMs,
	}
}

func defaultEQ() mix.EQ {
	if effect, ok := mix.DefaultEffects().Find(mix.EffectEQ); ok && effect.EQ != nil {
		return *effect.EQ
	}
	return mix.EQ{}
}
