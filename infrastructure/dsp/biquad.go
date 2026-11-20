// Package dsp implements the real-time effect processors behind the mix
// processor port: equalizer, compressor, drive, cabinet and reverb.
package dsp

import "math"

// biquad is a transposed-direct-form-II second order section.
type biquad struct {
	b0, b1, b2 float64
	a1, a2     float64
	z1, z2     float64
}

func (b *biquad) process(sample float64) float64 {
	output := b.b0*sample + b.z1
	b.z1 = b.b1*sample - b.a1*output + b.z2
	b.z2 = b.b2*sample - b.a2*output
	return output
}

func (b *biquad) reset() {
	b.z1, b.z2 = 0, 0
}

func (b *biquad) setPeaking(sampleRate, frequency, quality, gainDB float64) {
	amplitude := math.Pow(10, gainDB/40)
	omega := 2 * math.Pi * clampFrequency(frequency, sampleRate) / sampleRate
	alpha := math.Sin(omega) / (2 * quality)
	cosine := math.Cos(omega)
	normalize(b, 1+alpha*amplitude, -2*cosine, 1-alpha*amplitude,
		1+alpha/amplitude, -2*cosine, 1-alpha/amplitude)
}

func (b *biquad) setLowShelf(sampleRate, frequency, gainDB float64) {
	amplitude := math.Pow(10, gainDB/40)
	omega := 2 * math.Pi * clampFrequency(frequency, sampleRate) / sampleRate
	cosine := math.Cos(omega)
	alpha := math.Sin(omega) / 2 * math.Sqrt2
	root := math.Sqrt(amplitude)
	b0 := amplitude * ((amplitude + 1) - (amplitude-1)*cosine + 2*root*alpha)
	b1 := 2 * amplitude * ((amplitude - 1) - (amplitude+1)*cosine)
	b2 := amplitude * ((amplitude + 1) - (amplitude-1)*cosine - 2*root*alpha)
	a0 := (amplitude + 1) + (amplitude-1)*cosine + 2*root*alpha
	a1 := -2 * ((amplitude - 1) + (amplitude+1)*cosine)
	a2 := (amplitude + 1) + (amplitude-1)*cosine - 2*root*alpha
	normalize(b, b0, b1, b2, a0, a1, a2)
}

func (b *biquad) setHighShelf(sampleRate, frequency, gainDB float64) {
	amplitude := math.Pow(10, gainDB/40)
	omega := 2 * math.Pi * clampFrequency(frequency, sampleRate) / sampleRate
	cosine := math.Cos(omega)
	alpha := math.Sin(omega) / 2 * math.Sqrt2
	root := math.Sqrt(amplitude)
	b0 := amplitude * ((amplitude + 1) + (amplitude-1)*cosine + 2*root*alpha)
	b1 := -2 * amplitude * ((amplitude - 1) + (amplitude+1)*cosine)
	b2 := amplitude * ((amplitude + 1) + (amplitude-1)*cosine - 2*root*alpha)
	a0 := (amplitude + 1) - (amplitude-1)*cosine + 2*root*alpha
	a1 := 2 * ((amplitude - 1) - (amplitude+1)*cosine)
	a2 := (amplitude + 1) - (amplitude-1)*cosine - 2*root*alpha
	normalize(b, b0, b1, b2, a0, a1, a2)
}

func (b *biquad) setLowPass(sampleRate, frequency, quality float64) {
	omega := 2 * math.Pi * clampFrequency(frequency, sampleRate) / sampleRate
	alpha := math.Sin(omega) / (2 * quality)
	cosine := math.Cos(omega)
	normalize(b, (1-cosine)/2, 1-cosine, (1-cosine)/2, 1+alpha, -2*cosine, 1-alpha)
}

func (b *biquad) setHighPass(sampleRate, frequency, quality float64) {
	omega := 2 * math.Pi * clampFrequency(frequency, sampleRate) / sampleRate
	alpha := math.Sin(omega) / (2 * quality)
	cosine := math.Cos(omega)
	normalize(b, (1+cosine)/2, -(1 + cosine), (1+cosine)/2, 1+alpha, -2*cosine, 1-alpha)
}

func normalize(b *biquad, b0, b1, b2, a0, a1, a2 float64) {
	b.b0 = b0 / a0
	b.b1 = b1 / a0
	b.b2 = b2 / a0
	b.a1 = a1 / a0
	b.a2 = a2 / a0
}

func clampFrequency(frequency, sampleRate float64) float64 {
	limit := sampleRate * 0.45
	if frequency < 10 {
		return 10
	}
	if frequency > limit {
		return limit
	}
	return frequency
}
