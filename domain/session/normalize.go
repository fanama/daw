package session

import "math"

// NormalizeTargetDB is the peak level a normalized take aims for, one decibel
// below full scale.
const NormalizeTargetDB = -1.0

// NormalizeGainDB returns the take gain that moves a file's loudest peak to
// the target, clamped to the take gain range. It reports false for silence,
// which has no meaningful normalization.
func NormalizeGainDB(peak float32, targetDB float64) (float64, bool) {
	if peak <= 0 || math.IsNaN(float64(peak)) || math.IsInf(float64(peak), 0) {
		return 0, false
	}
	gain := targetDB - 20*math.Log10(float64(peak))
	return math.Max(-24, math.Min(24, gain)), true
}
