// Package recording implements the recording use case: a lock-free sink from
// the audio callback, a writer goroutine, and playback preloading.
package recording

import (
	"sync"
	"sync/atomic"

	"fanamix/domain/studio"
)

type frameBlock struct {
	data   []float32
	frames int
}

// frameSink hands interleaved frames from the audio callback to the writer
// goroutine. Offer never blocks: if the pipeline is saturated it drops the
// block and counts a stall.
type frameSink struct {
	blockSamples int
	blocks       chan frameBlock
	pool         chan []float32
	stalls       atomic.Int64
}

func newFrameSink(blockSamples, blockCount int) *frameSink {
	if blockSamples < 1 {
		blockSamples = 1
	}
	sink := &frameSink{
		blockSamples: blockSamples,
		blocks:       make(chan frameBlock, blockCount),
		pool:         make(chan []float32, blockCount),
	}
	for index := 0; index < blockCount; index++ {
		sink.pool <- make([]float32, blockSamples)
	}
	return sink
}

// Offer copies the frames into a pooled block and queues it.
func (s *frameSink) Offer(frames []float32) {
	if len(frames) == 0 {
		return
	}
	if len(frames) > s.blockSamples {
		frames = frames[:s.blockSamples]
	}
	select {
	case block := <-s.pool:
		copy(block, frames)
		select {
		case s.blocks <- frameBlock{data: block, frames: len(frames)}:
		default:
			s.stalls.Add(1)
			s.pool <- block
		}
	default:
		s.stalls.Add(1)
	}
}

// peakCollector folds interleaved frames into min/max buckets for the live
// waveform. It runs on the writer goroutine, never the audio thread.
type peakCollector struct {
	mu           sync.Mutex
	buckets      []studio.PeakBucket
	bucketFrames int
	pending      int
	min          float32
	max          float32
}

func newPeakCollector(bucketFrames int) *peakCollector {
	if bucketFrames < 1 {
		bucketFrames = 256
	}
	return &peakCollector{bucketFrames: bucketFrames, min: 1, max: -1}
}

func (p *peakCollector) add(frames []float32, channels int) {
	if channels < 1 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for offset := 0; offset+channels <= len(frames); offset += channels {
		loudest := float32(0)
		for channel := 0; channel < channels; channel++ {
			value := frames[offset+channel]
			if value < 0 {
				value = -value
			}
			if value > loudest {
				loudest = value
			}
		}
		if loudest > p.max {
			p.max = loudest
		}
		if -loudest < p.min {
			p.min = -loudest
		}
		p.pending++
		if p.pending >= p.bucketFrames {
			p.flushLocked()
		}
	}
}

func (p *peakCollector) snapshot() []studio.PeakBucket {
	p.mu.Lock()
	defer p.mu.Unlock()
	frozen := make([]studio.PeakBucket, len(p.buckets))
	copy(frozen, p.buckets)
	return frozen
}

func (p *peakCollector) flushLocked() {
	if p.pending == 0 {
		return
	}
	minValue, maxValue := p.min, p.max
	if minValue > maxValue {
		minValue, maxValue = 0, 0
	}
	p.buckets = append(p.buckets, studio.PeakBucket{Min: minValue, Max: maxValue})
	p.pending = 0
	p.min, p.max = 1, -1
}
