package dsp

import "fanamix/domain/mix"

// cabStage approximates a guitar speaker cabinet: no low end below ~80 Hz, a
// resonant body, and a rolled-off top. It is deliberately a fixed filter so a
// convolution cabinet can replace it behind the same chain slot later.
type cabStage struct {
	enabled  bool
	highPass biquad
	lowPass  biquad
	presence biquad
}

func (c *cabStage) configure(sampleRate int, settings mix.Cab) {
	c.enabled = settings.Enabled
	rate := float64(sampleRate)
	cutoff := 3800 + settings.Character*3400
	c.highPass.setHighPass(rate, 80, 0.707)
	c.lowPass.setLowPass(rate, cutoff, 0.8)
	c.presence.setHighShelf(rate, 2800, 1.5+settings.Character*2.5)
}

func (c *cabStage) processMono(samples []float32) {
	if !c.enabled {
		return
	}
	for index, sample := range samples {
		value := c.highPass.process(float64(sample))
		value = c.lowPass.process(value)
		value = c.presence.process(value)
		samples[index] = float32(value)
	}
}

func (c *cabStage) reset() {
	c.highPass.reset()
	c.lowPass.reset()
	c.presence.reset()
}
