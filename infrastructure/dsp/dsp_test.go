package dsp_test

import (
	"math"
	"testing"

	"fanamix/domain/mix"
	"fanamix/infrastructure/dsp"
)

func measureGainDB(t *testing.T, chain *dsp.Chain, frequency, sampleRate float64) float64 {
	t.Helper()
	const frames = 8192
	buffer := make([]float32, frames)
	for index := range buffer {
		buffer[index] = float32(0.1 * math.Sin(2*math.Pi*frequency*float64(index)/sampleRate))
	}
	chain.ProcessMono(buffer)
	sum := 0.0
	for _, sample := range buffer[frames/2:] {
		sum += float64(sample) * float64(sample)
	}
	rms := math.Sqrt(sum / float64(frames/2))
	return 20 * math.Log10(rms/0.0707)
}

func TestEQBandAddsExpectedGain(t *testing.T) {
	effects := mix.DefaultEffects()
	effects.Amp.Enabled = false
	effects.Cab.Enabled = false
	effects.EQ.Enabled = true
	effects.EQ.Bands[1] = mix.EQBand{FrequencyHz: 1000, Q: 1.0, GainDB: 12}
	effects.Comp.Enabled = false
	effects.Reverb.Enabled = false

	chain := dsp.NewChain()
	chain.Prepare(48000)
	chain.SetEffects(effects)

	gain := measureGainDB(t, chain, 1000, 48000)
	if gain < 10 || gain > 14 {
		t.Fatalf("expected about 12 dB at the band center, got %.2f dB", gain)
	}
}

func TestCompressorReducesLoudBlocks(t *testing.T) {
	effects := mix.DefaultEffects()
	effects.Amp.Enabled = false
	effects.Cab.Enabled = false
	effects.EQ.Enabled = false
	effects.Reverb.Enabled = false
	effects.Comp = mix.Compressor{
		Enabled: true, ThresholdDB: -30, Ratio: 8, AttackMs: 1, ReleaseMs: 100, KneeDB: 2,
	}
	chain := dsp.NewChain()
	chain.Prepare(48000)
	chain.SetEffects(effects)

	buffer := make([]float32, 4800)
	for index := range buffer {
		buffer[index] = float32(0.5 * math.Sin(2*math.Pi*220*float64(index)/48000))
	}
	chain.ProcessMono(buffer)
	if chain.GainReductionDB() < 6 {
		t.Fatalf("expected at least 6 dB of gain reduction, got %.2f", chain.GainReductionDB())
	}
}

func TestChainIsAllocationFreeAfterPrepare(t *testing.T) {
	chain := dsp.NewChain()
	chain.Prepare(48000)
	chain.SetEffects(mix.DefaultEffects())
	buffer := make([]float32, 256)
	allocations := testing.AllocsPerRun(50, func() {
		chain.ProcessMono(buffer)
	})
	if allocations != 0 {
		t.Fatalf("expected zero allocations in the audio path, got %.2f", allocations)
	}
}

func TestMetronomeAccentsTheDownbeat(t *testing.T) {
	metronome := dsp.NewMetronome()
	metronome.Configure(mix.Click{Enabled: true, BPM: 120, BeatsPerBar: 4}, 48000)

	downbeat := make([]float32, 512)
	metronome.Render(downbeat, 0)
	offbeat := make([]float32, 512)
	metronome.Render(offbeat, 24000)

	if peak(downbeat) <= peak(offbeat) {
		t.Fatalf("downbeat should be louder: %.3f vs %.3f", peak(downbeat), peak(offbeat))
	}
	disabled := dsp.NewMetronome()
	disabled.Configure(mix.Click{Enabled: false, BPM: 120, BeatsPerBar: 4}, 48000)
	silent := make([]float32, 512)
	disabled.Render(silent, 0)
	if peak(silent) != 0 {
		t.Fatal("a disabled click must write nothing")
	}
}

func peak(samples []float32) float32 {
	maximum := float32(0)
	for _, sample := range samples {
		if value := float32(math.Abs(float64(sample))); value > maximum {
			maximum = value
		}
	}
	return maximum
}
