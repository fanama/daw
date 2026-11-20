package malgoengine

import (
	"math"
	"time"
)

const (
	pitchWindowSeconds = 0.085
	pitchInterval      = 120 * time.Millisecond
	minimumFrequency   = 55.0
	maximumFrequency   = 1400.0
	pitchConfidence    = 0.6
	pitchGate          = 0.002
)

func (e *Engine) startTuner() {
	e.tunerDone = make(chan struct{})
	done := e.tunerDone
	go func() {
		ticker := time.NewTicker(pitchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				e.analyzePitch()
			}
		}
	}()
}

// writeTuner copies the newest dry samples into the lock-free ring so the
// analyzer can read them without touching the callback. The write index is
// published once per block instead of once per sample.
func (e *Engine) writeTuner(frames int) {
	if len(e.dry) == 0 || len(e.tunerSamples) == 0 {
		return
	}
	source := e.dry[0][:frames]
	size := int64(len(e.tunerSamples))
	write := e.tunerWrite.Load()
	for _, sample := range source {
		e.tunerSamples[write%size].Store(math.Float32bits(sample))
		write++
	}
	e.tunerWrite.Store(write)
}

func (e *Engine) analyzePitch() {
	windowLength := int(pitchWindowSeconds * float64(e.config.SampleRate))
	if windowLength <= 0 {
		return
	}
	if windowLength > len(e.tunerSamples) {
		windowLength = len(e.tunerSamples)
	}
	window := make([]float32, windowLength)
	size := int64(len(e.tunerSamples))
	end := e.tunerWrite.Load()
	for index := 0; index < windowLength; index++ {
		position := (end - int64(windowLength) + int64(index)) % size
		if position < 0 {
			position += size
		}
		window[index] = math.Float32frombits(e.tunerSamples[position].Load())
	}
	frequency, confident := detectPitch(window, float64(e.config.SampleRate))
	e.pitchHz.Store(math.Float64bits(frequency))
	e.pitchHas.Store(confident)
}

// detectPitch uses normalized autocorrelation with parabolic interpolation.
func detectPitch(samples []float32, sampleRate float64) (float64, bool) {
	power := signalPower(samples)
	if math.Sqrt(power/float64(len(samples))) < pitchGate {
		return 0, false
	}
	minLag, maxLag := pitchLagRange(len(samples), sampleRate)
	if maxLag <= minLag {
		return 0, false
	}
	scores, bestLag, bestScore := correlationScores(samples, minLag, maxLag, power)
	if bestLag == 0 || bestScore < pitchConfidence {
		return 0, false
	}
	return sampleRate / refineLag(scores, bestLag, minLag, maxLag), true
}

func signalPower(samples []float32) float64 {
	total := 0.0
	for _, sample := range samples {
		total += float64(sample) * float64(sample)
	}
	return total
}

func pitchLagRange(length int, sampleRate float64) (int, int) {
	minLag := int(sampleRate / maximumFrequency)
	maxLag := int(sampleRate / minimumFrequency)
	if minLag < 2 || maxLag >= length/2 {
		return 1, 0
	}
	return minLag, maxLag
}

func correlationScores(samples []float32, minLag, maxLag int, power float64) ([]float64, int, float64) {
	scores := make([]float64, maxLag+1)
	bestLag, bestScore := 0, 0.0
	for lag := minLag; lag <= maxLag; lag++ {
		correlation := 0.0
		normalizer := 0.0
		for index := 0; index+lag < len(samples); index++ {
			correlation += float64(samples[index]) * float64(samples[index+lag])
			normalizer += float64(samples[index+lag]) * float64(samples[index+lag])
		}
		if normalizer > 0 {
			correlation /= math.Sqrt(normalizer * power)
		}
		scores[lag] = correlation
		if correlation > bestScore {
			bestScore = correlation
			bestLag = lag
		}
	}
	return scores, bestLag, bestScore
}

func refineLag(scores []float64, bestLag, minLag, maxLag int) float64 {
	refined := float64(bestLag)
	if bestLag <= minLag || bestLag >= maxLag {
		return refined
	}
	left, center, right := scores[bestLag-1], scores[bestLag], scores[bestLag+1]
	denominator := left - 2*center + right
	if denominator == 0 {
		return refined
	}
	return refined + 0.5*(left-right)/denominator
}
