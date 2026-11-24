package dsp

import "fanamix/domain/mix"

type eqStage struct {
	bands   [5]biquad
	enabled bool
}

func (e *eqStage) configure(sampleRate int, settings mix.EQ) {
	e.enabled = settings.Enabled
	for index, band := range settings.Bands {
		e.bands[index].setPeaking(
			float64(sampleRate),
			band.FrequencyHz,
			band.Q,
			band.GainDB,
		)
	}
}

func (e *eqStage) processMono(samples []float32) {
	if !e.enabled {
		return
	}
	for index, sample := range samples {
		value := float64(sample)
		for band := range e.bands {
			value = e.bands[band].process(value)
		}
		samples[index] = float32(value)
	}
}

func (e *eqStage) reset() {
	for band := range e.bands {
		e.bands[band].reset()
	}
}
