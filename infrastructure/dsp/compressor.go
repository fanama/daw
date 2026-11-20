package dsp

import (
	"math"

	"gorecord/domain/mix"
)

type compressorStage struct {
	enabled   bool
	threshold float64
	ratio     float64
	knee      float64
	makeup    float64
	attack    float64
	release   float64
	envelope  float64
	reduction float32
}

func (c *compressorStage) configure(sampleRate int, settings mix.Compressor) {
	c.enabled = settings.Enabled
	c.threshold = settings.ThresholdDB
	c.ratio = math.Max(settings.Ratio, 1)
	c.knee = math.Max(settings.KneeDB, 0.01)
	c.makeup = math.Pow(10, settings.MakeupDB/20)
	attackSeconds := math.Max(settings.AttackMs, 0.1) / 1000
	releaseSeconds := math.Max(settings.ReleaseMs, 1) / 1000
	c.attack = math.Exp(-1 / (float64(sampleRate) * attackSeconds))
	c.release = math.Exp(-1 / (float64(sampleRate) * releaseSeconds))
}

func (c *compressorStage) processMono(samples []float32) {
	if !c.enabled {
		c.reduction = 0
		return
	}
	maximumReduction := 0.0
	slope := 1 - 1/c.ratio
	for index, sample := range samples {
		level := math.Abs(float64(sample))
		coefficient := c.attack
		if level < c.envelope {
			coefficient = c.release
		}
		c.envelope = coefficient*c.envelope + (1-coefficient)*level

		decibels := 20 * math.Log10(c.envelope+1e-9)
		over := decibels - c.threshold
		reductionDB := c.softKneeReduction(over, slope)
		if -reductionDB > maximumReduction {
			maximumReduction = -reductionDB
		}
		gain := math.Pow(10, reductionDB/20) * c.makeup
		samples[index] = float32(float64(sample) * gain)
	}
	c.reduction = float32(maximumReduction)
}

func (c *compressorStage) softKneeReduction(over, slope float64) float64 {
	if over <= -c.knee/2 {
		return 0
	}
	if over >= c.knee/2 {
		return -over * slope
	}
	position := over + c.knee/2
	return -slope * position * position / (2 * c.knee)
}

func (c *compressorStage) gainReduction() float32 { return c.reduction }

func (c *compressorStage) reset() {
	c.envelope = 0
	c.reduction = 0
}
