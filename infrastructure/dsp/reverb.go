package dsp

import "gorecord/domain/mix"

var combTunings = [4]int{1116, 1188, 1277, 1356}
var allPassTunings = [2]int{556, 441}

type combFilter struct {
	buffer   []float64
	index    int
	feedback float64
	damp     float64
	store    float64
}

func (c *combFilter) prepare(length int) {
	c.buffer = make([]float64, length)
}

func (c *combFilter) process(input float64) float64 {
	output := c.buffer[c.index]
	c.store = output*(1-c.damp) + c.store*c.damp
	c.buffer[c.index] = input + c.store*c.feedback
	c.index++
	if c.index >= len(c.buffer) {
		c.index = 0
	}
	return output
}

type allPassFilter struct {
	buffer []float64
	index  int
}

func (a *allPassFilter) prepare(length int) {
	a.buffer = make([]float64, length)
}

func (a *allPassFilter) process(input float64) float64 {
	buffered := a.buffer[a.index]
	output := buffered - input
	a.buffer[a.index] = input + buffered*0.5
	a.index++
	if a.index >= len(a.buffer) {
		a.index = 0
	}
	return output
}

type reverbStage struct {
	enabled  bool
	combs    [4]combFilter
	allPass  [2]allPassFilter
	preDelay []float64
	preIndex int
	wet      float64
	dry      float64
}

func (r *reverbStage) prepare(sampleRate int) {
	scale := float64(sampleRate) / 44100
	for index := range r.combs {
		r.combs[index].prepare(int(float64(combTunings[index]) * scale))
	}
	for index := range r.allPass {
		r.allPass[index].prepare(int(float64(allPassTunings[index]) * scale))
	}
	r.preDelay = make([]float64, int(0.1*float64(sampleRate)))
}

func (r *reverbStage) configure(settings mix.Reverb) {
	r.enabled = settings.Enabled
	r.wet = settings.Mix
	r.dry = 1 - settings.Mix
	feedback := 0.7 + settings.Size*0.28
	for index := range r.combs {
		r.combs[index].feedback = feedback
		r.combs[index].damp = settings.Damp * 0.4
	}
	r.applyPreDelayLength(settings.PreDelayMs)
}

// applyPreDelayLength moves the read tap by zeroing the buffer, which is
// cheaper than reallocating inside the audio thread.
func (r *reverbStage) applyPreDelayLength(preDelayMs float64) {
	if len(r.preDelay) == 0 {
		return
	}
	frames := int(float64(len(r.preDelay)) / 100 * preDelayMs)
	if frames >= len(r.preDelay) {
		frames = len(r.preDelay) - 1
	}
	r.preIndex = frames
}

func (r *reverbStage) processMono(samples []float32) {
	if !r.enabled {
		return
	}
	for index, sample := range samples {
		dry := float64(sample)
		delayed := dry
		if len(r.preDelay) > 0 {
			delayed = r.preDelay[r.preIndex]
			r.preDelay[r.preIndex] = dry
			r.preIndex++
			if r.preIndex >= len(r.preDelay) {
				r.preIndex = 0
			}
		}
		wet := 0.0
		for comb := range r.combs {
			wet += r.combs[comb].process(delayed)
		}
		wet /= float64(len(r.combs))
		for pass := range r.allPass {
			wet = r.allPass[pass].process(wet)
		}
		samples[index] = float32(dry*r.dry + wet*r.wet)
	}
}

func (r *reverbStage) reset() {
	for index := range r.combs {
		for position := range r.combs[index].buffer {
			r.combs[index].buffer[position] = 0
		}
		r.combs[index].store = 0
	}
	for index := range r.allPass {
		for position := range r.allPass[index].buffer {
			r.allPass[index].buffer[position] = 0
		}
	}
	for position := range r.preDelay {
		r.preDelay[position] = 0
	}
}
