package dsp

import (
	"sync/atomic"

	"fanamix/domain/mix"
)

// Chain is the per-input-channel effect chain. It keeps one preallocated stage
// slot per chain position, so reordering effects or repeating a kind never
// allocates on the audio thread.
type Chain struct {
	sampleRate  int
	pending     atomic.Pointer[mix.Effects]
	applied     *mix.Effects
	slots       [mix.MaxEffectInstances]chainSlot
	active      [mix.MaxEffectInstances]int
	order       [mix.MaxEffectInstances]mix.EffectKind
	activeCount int
	compSlot    int
}

// chainSlot holds every stage a single chain position could need. Only the
// stage matching the position's effect kind is configured and processed.
type chainSlot struct {
	amp    driveStage
	cab    cabStage
	eq     eqStage
	comp   compressorStage
	reverb reverbStage
}

// NewChain returns a chain with the default effect settings.
func NewChain() *Chain {
	defaults := mix.DefaultEffects()
	chain := &Chain{compSlot: -1}
	chain.pending.Store(&defaults)
	return chain
}

// Prepare allocates delay memory and applies the pending settings.
func (c *Chain) Prepare(sampleRate int) {
	c.sampleRate = sampleRate
	for index := range c.slots {
		c.slots[index].reverb.prepare(sampleRate)
	}
	c.applied = nil
	c.applyPending()
}

// SetEffects publishes a new chain for the next block.
func (c *Chain) SetEffects(effects mix.Effects) {
	normalized := effects.Normalize()
	c.pending.Store(&normalized)
}

// ProcessMono filters one mono block in place, in chain order.
func (c *Chain) ProcessMono(samples []float32) {
	c.applyPending()
	for index := 0; index < c.activeCount; index++ {
		slot := &c.slots[c.active[index]]
		switch c.order[index] {
		case mix.EffectAmp:
			slot.amp.processMono(samples)
		case mix.EffectCab:
			slot.cab.processMono(samples)
		case mix.EffectEQ:
			slot.eq.processMono(samples)
		case mix.EffectComp:
			slot.comp.processMono(samples)
		case mix.EffectReverb:
			slot.reverb.processMono(samples)
		}
	}
}

// GainReductionDB reports the compressor gain reduction of the last block.
func (c *Chain) GainReductionDB() float32 {
	if c.compSlot < 0 {
		return 0
	}
	return c.slots[c.compSlot].comp.gainReduction()
}

// Reset clears every filter and delay line.
func (c *Chain) Reset() {
	for index := range c.slots {
		c.slots[index].amp.reset()
		c.slots[index].cab.reset()
		c.slots[index].eq.reset()
		c.slots[index].comp.reset()
		c.slots[index].reverb.reset()
	}
}

func (c *Chain) applyPending() {
	pending := c.pending.Load()
	if pending == nil || pending == c.applied {
		return
	}
	c.applied = pending
	c.activeCount = 0
	c.compSlot = -1
	for index, effect := range pending.Chain {
		if index >= len(c.slots) {
			break
		}
		slot := &c.slots[index]
		switch effect.Kind {
		case mix.EffectAmp:
			if effect.Amp == nil {
				continue
			}
			slot.amp.configure(c.sampleRate, *effect.Amp)
		case mix.EffectCab:
			if effect.Cab == nil {
				continue
			}
			slot.cab.configure(c.sampleRate, *effect.Cab)
		case mix.EffectEQ:
			if effect.EQ == nil {
				continue
			}
			slot.eq.configure(c.sampleRate, *effect.EQ)
		case mix.EffectComp:
			if effect.Comp == nil {
				continue
			}
			slot.comp.configure(c.sampleRate, *effect.Comp)
			if effect.Comp.Enabled {
				c.compSlot = index
			}
		case mix.EffectReverb:
			if effect.Reverb == nil {
				continue
			}
			slot.reverb.configure(*effect.Reverb)
		default:
			continue
		}
		c.active[c.activeCount] = index
		c.order[c.activeCount] = effect.Kind
		c.activeCount++
	}
}

// Factory creates chain processors for the audio adapter.
type Factory struct{}

// NewProcessor returns a prepared-on-first-use chain.
func (f Factory) NewProcessor() mix.Processor { return NewChain() }
