// Chromatic tuner: normalized autocorrelation with parabolic interpolation,
// run on a short pre-effects window a few times per second.

const NOTES = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"];

function energy(samples) {
  let total = 0;
  for (let index = 0; index < samples.length; index++) {
    total += samples[index] * samples[index];
  }
  return total / samples.length;
}

function correlationAt(samples, lag) {
  let sum = 0;
  let norm = 0;
  const limit = samples.length - lag;
  for (let index = 0; index < limit; index++) {
    sum += samples[index] * samples[index + lag];
    norm += samples[index] * samples[index];
  }
  return norm > 0 ? sum / norm : 0;
}

export function detectPitch(samples, sampleRate) {
  if (energy(samples) < 1e-5) {
    return 0;
  }
  const minLag = Math.floor(sampleRate / 1200);
  const maxLag = Math.min(Math.floor(sampleRate / 55), Math.floor(samples.length / 2));
  if (maxLag <= minLag) {
    return 0;
  }
  const scores = new Float32Array(maxLag + 2);
  let bestLag = 0;
  let best = 0;
  for (let lag = minLag; lag <= maxLag; lag++) {
    scores[lag] = correlationAt(samples, lag);
    if (scores[lag] > best) {
      best = scores[lag];
      bestLag = lag;
    }
  }
  if (bestLag === 0 || best < 0.25) {
    return 0;
  }
  const previous = scores[bestLag - 1] || 0;
  const next = scores[bestLag + 1] || 0;
  const denominator = previous - 2 * best + next;
  const shift = denominator === 0 ? 0 : (0.5 * (previous - next)) / denominator;
  return sampleRate / (bestLag + shift);
}

export function noteReading(frequency) {
  if (!frequency || frequency < 40 || frequency > 2500) {
    return null;
  }
  const exact = 69 + 12 * Math.log2(frequency / 440);
  const midi = Math.round(exact);
  const cents = Math.round((exact - midi) * 100);
  const octave = Math.floor(midi / 12) - 1;
  return { note: `${NOTES[((midi % 12) + 12) % 12]}${octave}`, cents };
}
