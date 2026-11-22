// Clock ratios for sources captured on independent devices. A probe reports
// cumulative source frames; the slope against wall time gives the actual rate.

const MIN_RATIO = 0.98;
const MAX_RATIO = 1.02;
const MIN_WINDOW_SECONDS = 0.75;

export function estimateClockRatio(samples, nominalRate) {
  if (!Array.isArray(samples) || !Number.isFinite(nominalRate) || nominalRate <= 0) return 1;
  const usable = samples
    .filter((sample) => Number.isFinite(sample?.frames) && Number.isFinite(sample?.at))
    .sort((left, right) => left.at - right.at);
  if (usable.length < 2) return 1;
  const start = usable[0];
  const elapsed = (usable[usable.length - 1].at - start.at) / 1000;
  if (elapsed < MIN_WINDOW_SECONDS) return 1;
  const slope = frameSlope(usable, start);
  if (!Number.isFinite(slope) || slope <= 0) return 1;
  return Math.min(MAX_RATIO, Math.max(MIN_RATIO, slope / nominalRate));
}

function frameSlope(samples, start) {
  let count = 0;
  let sumX = 0;
  let sumY = 0;
  let sumXX = 0;
  let sumXY = 0;
  for (const sample of samples) {
    const x = (sample.at - start.at) / 1000;
    const y = sample.frames - start.frames;
    count += 1;
    sumX += x;
    sumY += y;
    sumXX += x * x;
    sumXY += x * y;
  }
  const denominator = count * sumXX - sumX * sumX;
  if (denominator <= 1e-9) return 0;
  return (count * sumXY - sumX * sumY) / denominator;
}
