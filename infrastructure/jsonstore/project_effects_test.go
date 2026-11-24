package jsonstore

import (
	"testing"

	"fanamix/domain/mix"
)

func TestEffectsToDomainMigratesThreeBandEQ(t *testing.T) {
	stored := effectsJSON{
		EQ: &eqJSON{
			Enabled: true,
			Bands: []bandJSON{
				{FrequencyHz: 100, Q: 0.7, GainDB: -4},
				{FrequencyHz: 1000, Q: 0.9, GainDB: 2},
				{FrequencyHz: 6000, Q: 0.7, GainDB: 5},
			},
		},
	}
	effect, ok := effectsToDomain(stored).Find(mix.EffectEQ)
	if !ok || effect.EQ == nil {
		t.Fatal("legacy EQ was dropped")
	}
	bands := effect.EQ.Bands
	if bands[0].FrequencyHz != 20 || bands[4].FrequencyHz != 20000 {
		t.Fatalf("legacy EQ did not gain endpoint bands: %+v", bands)
	}
	if bands[1].FrequencyHz != 100 || bands[1].GainDB != -4 ||
		bands[2].FrequencyHz != 1000 || bands[2].GainDB != 2 ||
		bands[3].FrequencyHz != 6000 || bands[3].GainDB != 5 {
		t.Fatalf("legacy EQ bands are misaligned: %+v", bands)
	}
	if bands[0].GainDB != 0 || bands[4].GainDB != 0 {
		t.Fatalf("endpoint bands must start flat: %+v", bands)
	}
}

func TestEffectsFromDomainWritesFiveBands(t *testing.T) {
	stored := effectsFromDomain(mix.DefaultEffects())
	var eq *eqJSON
	for index := range stored.Chain {
		if stored.Chain[index].Kind == string(mix.EffectEQ) {
			eq = stored.Chain[index].EQ
		}
	}
	if eq == nil || len(eq.Bands) != 5 {
		t.Fatalf("expected one stored five-band equalizer, got %+v", stored.Chain)
	}
}

func TestEffectsRoundTripKeepsOrderAndRepeats(t *testing.T) {
	effects := mix.Effects{Chain: []mix.Effect{
		mix.NewEffect(mix.EffectReverb),
		mix.NewEffect(mix.EffectEQ),
		mix.NewEffect(mix.EffectReverb),
	}}
	loaded := effectsToDomain(effectsFromDomain(effects))
	if len(loaded.Chain) != 3 {
		t.Fatalf("expected three instances, got %+v", loaded.Chain)
	}
	if loaded.Chain[0].Kind != mix.EffectReverb || loaded.Chain[1].Kind != mix.EffectEQ || loaded.Chain[2].Kind != mix.EffectReverb {
		t.Fatalf("chain order changed: %+v", loaded.Chain)
	}
	if loaded.Chain[0].Reverb == nil || loaded.Chain[2].Reverb == nil {
		t.Fatalf("repeated instances lost their settings: %+v", loaded.Chain)
	}
}

func TestEffectsToDomainKeepsAnEmptyChain(t *testing.T) {
	loaded := effectsToDomain(effectsJSON{Chain: []effectJSON{}})
	if len(loaded.Chain) != 0 || !loaded.Bypassed() {
		t.Fatalf("an explicit empty chain must stay empty: %+v", loaded.Chain)
	}
}
