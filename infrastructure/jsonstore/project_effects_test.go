package jsonstore

import (
	"testing"

	"fanamix/domain/mix"
)

func TestEffectsToDomainMigratesThreeBandEQ(t *testing.T) {
	stored := effectsJSON{
		EQ: eqJSON{
			Enabled: true,
			Bands: []bandJSON{
				{FrequencyHz: 100, Q: 0.7, GainDB: -4},
				{FrequencyHz: 1000, Q: 0.9, GainDB: 2},
				{FrequencyHz: 6000, Q: 0.7, GainDB: 5},
			},
		},
	}
	bands := effectsToDomain(stored).EQ.Bands
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
	if len(stored.EQ.Bands) != 5 {
		t.Fatalf("expected five stored bands, got %d", len(stored.EQ.Bands))
	}
}
