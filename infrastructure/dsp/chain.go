package dsp

import (
	"sync/atomic"

	"fanamix/domain/mix"
)

// Chain is the per-input-channel effect chain. Parameter changes arrive
// through an atomic pointer and are applied at block boundaries, so the audio
// thread never locks or allocates.
type Chain struct {
	sampleRate int
	pending    atomic.Pointer[mix.Effects]
	applied    mix.Effects
	hasApplied bool
	amp        driveStage
	cab        cabStage
	eq         eqStage
	comp       compressorStage
	reverb     reverbStage
}

// NewChain returns a chain with the default effect settings.
func NewChain() *Chain {
	defaults := mix.DefaultEffects()
	chain := &Chain{}
	chain.pending.Store(&defaults)
	return chain
}

// Prepare allocates delay memory and applies the pending settings.
func (c *Chain) Prepare(sampleRate int) {
	c.sampleRate = sampleRate
	c.reverb.prepare(sampleRate)
	c.hasApplied = false
	c.applyPending()
}

// SetEffects publishes a new configuration for the next block.
func (c *Chain) SetEffects(effects mix.Effects) {
	normalized := effects.Normalize()
	c.pending.Store(&normalized)
}

// ProcessMono filters one mono block in place.
func (c *Chain) ProcessMono(samples []float32) {
	c.applyPending()
	c.amp.processMono(samples)
	c.cab.processMono(samples)
	c.eq.processMono(samples)
	c.comp.processMono(samples)
	c.reverb.processMono(samples)
}

// GainReductionDB reports the compressor gain reduction of the last block.
func (c *Chain) GainReductionDB() float32 { return c.comp.gainReduction() }

// Reset clears every filter and delay line.
func (c *Chain) Reset() {
	c.amp.reset()
	c.cab.reset()
	c.eq.reset()
	c.comp.reset()
	c.reverb.reset()
}

func (c *Chain) applyPending() {
	pending := c.pending.Load()
	if pending == nil || (c.hasApplied && *pending == c.applied) {
		return
	}
	c.applied = *pending
	c.hasApplied = true
	c.amp.configure(c.sampleRate, c.applied.Amp)
	c.cab.configure(c.sampleRate, c.applied.Cab)
	c.eq.configure(c.sampleRate, c.applied.EQ)
	c.comp.configure(c.sampleRate, c.applied.Comp)
	c.reverb.configure(c.applied.Reverb)
}

// Factory creates chain processors for the audio adapter.
type Factory struct{}

// NewProcessor returns a prepared-on-first-use chain.
func (f Factory) NewProcessor() mix.Processor { return NewChain() }
