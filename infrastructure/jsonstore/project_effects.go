package jsonstore

import "gorecord/domain/mix"

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

type effectsJSON struct {
	Amp    ampJSON        `json:"amp"`
	Cab    cabJSON        `json:"cab"`
	EQ     eqJSON         `json:"eq"`
	Comp   compressorJSON `json:"comp"`
	Reverb reverbJSON     `json:"reverb"`
}

func effectsFromDomain(effects mix.Effects) effectsJSON {
	bands := make([]bandJSON, 0, len(effects.EQ.Bands))
	for _, band := range effects.EQ.Bands {
		bands = append(bands, bandJSON{FrequencyHz: band.FrequencyHz, Q: band.Q, GainDB: band.GainDB})
	}
	return effectsJSON{
		Amp: ampJSON{
			Enabled:  effects.Amp.Enabled,
			Drive:    effects.Amp.Drive,
			BassDB:   effects.Amp.BassDB,
			MidDB:    effects.Amp.MidDB,
			TrebleDB: effects.Amp.TrebleDB,
			LevelDB:  effects.Amp.LevelDB,
		},
		Cab: cabJSON{Enabled: effects.Cab.Enabled, Character: effects.Cab.Character},
		EQ:  eqJSON{Enabled: effects.EQ.Enabled, Bands: bands},
		Comp: compressorJSON{
			Enabled:     effects.Comp.Enabled,
			ThresholdDB: effects.Comp.ThresholdDB,
			Ratio:       effects.Comp.Ratio,
			AttackMs:    effects.Comp.AttackMs,
			ReleaseMs:   effects.Comp.ReleaseMs,
			KneeDB:      effects.Comp.KneeDB,
			MakeupDB:    effects.Comp.MakeupDB,
		},
		Reverb: reverbJSON{
			Enabled:    effects.Reverb.Enabled,
			Size:       effects.Reverb.Size,
			Damp:       effects.Reverb.Damp,
			Mix:        effects.Reverb.Mix,
			PreDelayMs: effects.Reverb.PreDelayMs,
		},
	}
}

func effectsToDomain(stored effectsJSON) mix.Effects {
	effects := mix.DefaultEffects()
	effects.Amp = mix.Amp{
		Enabled:  stored.Amp.Enabled,
		Drive:    stored.Amp.Drive,
		BassDB:   stored.Amp.BassDB,
		MidDB:    stored.Amp.MidDB,
		TrebleDB: stored.Amp.TrebleDB,
		LevelDB:  stored.Amp.LevelDB,
	}
	effects.Cab = mix.Cab{Enabled: stored.Cab.Enabled, Character: stored.Cab.Character}
	effects.EQ.Enabled = stored.EQ.Enabled
	for index := 0; index < len(effects.EQ.Bands) && index < len(stored.EQ.Bands); index++ {
		effects.EQ.Bands[index] = mix.EQBand{
			FrequencyHz: stored.EQ.Bands[index].FrequencyHz,
			Q:           stored.EQ.Bands[index].Q,
			GainDB:      stored.EQ.Bands[index].GainDB,
		}
	}
	effects.Comp = mix.Compressor{
		Enabled:     stored.Comp.Enabled,
		ThresholdDB: stored.Comp.ThresholdDB,
		Ratio:       stored.Comp.Ratio,
		AttackMs:    stored.Comp.AttackMs,
		ReleaseMs:   stored.Comp.ReleaseMs,
		KneeDB:      stored.Comp.KneeDB,
		MakeupDB:    stored.Comp.MakeupDB,
	}
	effects.Reverb = mix.Reverb{
		Enabled:    stored.Reverb.Enabled,
		Size:       stored.Reverb.Size,
		Damp:       stored.Reverb.Damp,
		Mix:        stored.Reverb.Mix,
		PreDelayMs: stored.Reverb.PreDelayMs,
	}
	return effects.Normalize()
}
