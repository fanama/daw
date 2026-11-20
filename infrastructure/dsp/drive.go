package dsp

import (
	"math"

	"gorecord/domain/mix"
)

// oversamplingFactor is the number of interpolated subsamples fed to the
// waveshaper, which lowers aliasing without a full resampler.
const oversamplingFactor = 4

type driveStage struct {
	enabled  bool
	preGain  float64
	level    float64
	previous float64
	bass     biquad
	mid      biquad
	treble   biquad
	bias     float64
}

func (d *driveStage) configure(sampleRate int, settings mix.Amp) {
	d.enabled = settings.Enabled
	d.preGain = 1 + settings.Drive*36
	d.level = math.Pow(10, settings.LevelDB/20)
	d.bias = 0.08 + settings.Drive*0.12
	d.bass.setLowShelf(float64(sampleRate), 140, settings.BassDB)
	d.mid.setPeaking(float64(sampleRate), 850, 0.8, settings.MidDB)
	d.treble.setHighShelf(float64(sampleRate), 3200, settings.TrebleDB)
}

func (d *driveStage) processMono(samples []float32) {
	if !d.enabled {
		return
	}
	for index, sample := range samples {
		shaped := d.shapeOversampled(float64(sample))
		value := d.bass.process(shaped)
		value = d.mid.process(value)
		value = d.treble.process(value)
		samples[index] = float32(value * d.level)
	}
}

func (d *driveStage) shapeOversampled(sample float64) float64 {
	accumulated := 0.0
	for step := 1; step <= oversamplingFactor; step++ {
		interpolated := d.previous + (sample-d.previous)*float64(step)/oversamplingFactor
		accumulated += d.shape(interpolated * d.preGain)
	}
	d.previous = sample
	return accumulated / oversamplingFactor
}

func (d *driveStage) shape(value float64) float64 {
	return math.Tanh(value+d.bias) - math.Tanh(d.bias)
}

func (d *driveStage) reset() {
	d.previous = 0
	d.bass.reset()
	d.mid.reset()
	d.treble.reset()
}
