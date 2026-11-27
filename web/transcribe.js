// Audio-to-MIDI extraction for takes: a spectral pitch salience pass turns the
// samples into notes, then a quantized pass folds them into a single melody
// line. It is pure, so the page can run it between paints and the node check can
// feed it synthetic takes.

const NOTE_LOW = 36; // C2
const NOTE_HIGH = 95; // B6
const HARMONICS = 8;
const MAX_VOICES = 6;
// The window is long enough to place low notes: 4096 samples split the bass
// range into bins a semitone apart, which 2048 could not.
const WINDOW = 4096;
const MIN_RMS = 0.004;
const VOICE_SHARE = 0.4;
// A pitch whose own fundamental is missing is another note's harmonic, not a
// voice: this is what keeps a chord from being read an octave below.
const FUNDAMENTAL_SHARE = 0.08;
const OCTAVE = 12;
const HOLD_FRAMES = 2;
const MIN_FRAMES = 3;
const MAX_SECONDS = 180;
// The beat is read from the onsets themselves: a take played a little off the
// session tempo still has its own pulse, and notes should land on that one.
const TEMPO_LOW = 50;
const TEMPO_HIGH = 210;
const TEMPO_CENTER = 120;
const TEMPO_SPREAD = 0.9; // octaves of tolerance before the prior fades out
const MIN_ONSETS = 4;
const SUBDIVISION = 4; // sixteenths per beat

function toMono(samples, channels) {
  const count = Math.max(1, Math.round(Number(channels) || 1));
  if (count === 1) return samples;
  const frames = Math.floor(samples.length / count);
  const mono = new Float32Array(frames);
  for (let frame = 0; frame < frames; frame++) {
    let sum = 0;
    for (let channel = 0; channel < count; channel++) sum += samples[frame * count + channel];
    mono[frame] = sum / count;
  }
  return mono;
}

function reverseBits(value, bits) {
  let result = 0;
  for (let index = 0; index < bits; index++) {
    result = result * 2 + (value % 2);
    value = Math.floor(value / 2);
  }
  return result;
}

// createTransform returns an in-place iterative radix-2 FFT for a power-of-two
// size, with its twiddles and bit reversal prepared once.
function createTransform(size) {
  const levels = Math.log2(size);
  if (!Number.isInteger(levels)) throw new Error("FFT size must be a power of two");
  const cos = new Float32Array(size / 2);
  const sin = new Float32Array(size / 2);
  for (let index = 0; index < size / 2; index++) {
    cos[index] = Math.cos((2 * Math.PI * index) / size);
    sin[index] = Math.sin((2 * Math.PI * index) / size);
  }
  const reversed = new Uint32Array(size);
  for (let index = 0; index < size; index++) reversed[index] = reverseBits(index, levels);
  return (re, im) => {
    for (let index = 0; index < size; index++) {
      const target = reversed[index];
      if (target <= index) continue;
      const real = re[index];
      const imaginary = im[index];
      re[index] = re[target];
      im[index] = im[target];
      re[target] = real;
      im[target] = imaginary;
    }
    for (let span = 2; span <= size; span *= 2) {
      const half = span / 2;
      const stride = size / span;
      for (let start = 0; start < size; start += span) {
        for (let offset = 0; offset < half; offset++) {
          const twiddle = offset * stride;
          const re2 = re[start + offset + half] * cos[twiddle] + im[start + offset + half] * sin[twiddle];
          const im2 = im[start + offset + half] * cos[twiddle] - re[start + offset + half] * sin[twiddle];
          re[start + offset + half] = re[start + offset] - re2;
          im[start + offset + half] = im[start + offset] - im2;
          re[start + offset] += re2;
          im[start + offset] += im2;
        }
      }
    }
  };
}

function hannWindow(size) {
  const window = new Float32Array(size);
  for (let index = 0; index < size; index++) window[index] = 0.5 - 0.5 * Math.cos((2 * Math.PI * index) / size);
  return window;
}

// magnitudeAt reads the spectrum at a fractional bin. Interpolating instead of
// rounding keeps neighbouring low notes apart, where one bin covers more than a
// semitone and rounding would hand a note its neighbour's energy.
function magnitudeAt(magnitudes, bin) {
  if (bin < 1 || bin >= magnitudes.length - 1) return 0;
  const low = Math.floor(bin);
  const rest = bin - low;
  return magnitudes[low] * (1 - rest) + magnitudes[low + 1] * rest;
}

// pitchLevels scores every candidate pitch by its harmonic series, counting the
// fundamental in full and the harmonics by their inverse rank.
function pitchLevels(magnitudes, binHz) {
  const levels = new Float32Array(NOTE_HIGH + 1);
  for (let pitch = NOTE_LOW; pitch <= NOTE_HIGH; pitch++) {
    const fundamental = fundamentalOf(pitch);
    let total = 0;
    for (let harmonic = 1; harmonic <= HARMONICS; harmonic++) {
      const bin = (fundamental * harmonic) / binHz;
      if (bin >= magnitudes.length) break;
      total += (harmonic === 1 ? 1 : 1 / harmonic) * magnitudeAt(magnitudes, bin);
    }
    levels[pitch] = total;
  }
  return levels;
}

function fundamentalOf(pitch) {
  return 440 * 2 ** ((pitch - 69) / 12);
}

// isSpectralPeak tells a real partial from the low-frequency slope a note's
// attack leaves across the spectrum: that slope has no peak, yet its level at a
// bass note's fundamental is high enough to be read as a voice of its own.
function isSpectralPeak(magnitudes, bin) {
  const center = Math.round(bin);
  if (center < 2 || center >= magnitudes.length - 1) return false;
  const value = magnitudes[center];
  return value >= magnitudes[center - 1] && value >= magnitudes[center + 1];
}

// frameVoices keeps the local peaks of the pitch axis whose fundamental is
// really there, so neither an octave duplicate nor a harmonic series borrowed
// from the notes above answers for a note that is not sounding.
function frameVoices(levels, magnitudes, binHz) {
  const highest = levels.reduce((top, level) => Math.max(top, level), 0);
  const topMagnitude = magnitudes.reduce((top, value) => Math.max(top, value), 0);
  if (highest <= 0 || topMagnitude <= 0) return [];
  const floor = highest * VOICE_SHARE;
  const voices = [];
  for (let pitch = NOTE_LOW; pitch <= NOTE_HIGH; pitch++) {
    const level = levels[pitch];
    if (level < floor) continue;
    const lower = pitch > NOTE_LOW ? levels[pitch - 1] : 0;
    const upper = pitch < NOTE_HIGH ? levels[pitch + 1] : 0;
    if (level < lower || level < upper) continue;
    const bin = fundamentalOf(pitch) / binHz;
    if (magnitudeAt(magnitudes, bin) < topMagnitude * FUNDAMENTAL_SHARE) continue;
    if (!isSpectralPeak(magnitudes, bin)) continue;
    voices.push({ pitch, level });
  }
  // A note an octave above another one is usually that note's second harmonic,
  // so it only survives when it is the stronger of the two.
  const kept = voices.filter((voice) => {
    const lower = voices.find((other) => other.pitch === voice.pitch - OCTAVE);
    return !lower || lower.level < voice.level;
  });
  return kept.sort((left, right) => right.level - left.level).slice(0, MAX_VOICES);
}

// trackNotes follows the voices of successive frames into notes. A voice may
// drop out for a couple of frames before its note ends, which keeps vibrato and
// quiet harmonics from splitting a note in two.
function trackNotes(voicesPerFrame, hop, windowOffset, minFrames) {
  const notes = [];
  let open = [];
  voicesPerFrame.forEach((voices, frame) => {
    const claimed = new Set();
    for (const voice of voices) {
      const match = open.find((note) => !claimed.has(note) && Math.abs(note.pitch - voice.pitch) <= 1);
      if (match) {
        claimed.add(match);
        match.lastSeen = frame;
        match.pitch = voice.pitch;
        match.energy = Math.max(match.energy, voice.level);
        continue;
      }
      const fresh = { pitch: voice.pitch, start: frame, lastSeen: frame, energy: voice.level };
      open.push(fresh);
      claimed.add(fresh);
    }
    open = open.filter((note) => {
      if (frame - note.lastSeen <= HOLD_FRAMES) return true;
      notes.push(note);
      return false;
    });
  });
  notes.push(...open);
  const peak = notes.reduce((top, note) => Math.max(top, note.energy), 0);
  return notes
    .filter((note) => note.lastSeen - note.start + 1 >= minFrames)
    .map((note) => ({
      pitch: note.pitch,
      startFrame: note.start * hop + windowOffset,
      endFrame: note.lastSeen * hop + windowOffset + hop,
      velocity: peak > 0 ? Math.max(30, Math.min(120, Math.round(40 + 70 * Math.sqrt(note.energy / peak)))) : 100,
    }));
}

// onsetStrength is the rise of the pitch salience from one frame to the next.
// Held notes give nothing, so the curve marks attacks and nothing else, which
// is what the tempo and phase search needs.
function onsetStrength(levelsPerFrame) {
  const curve = new Float32Array(levelsPerFrame.length);
  for (let frame = 1; frame < levelsPerFrame.length; frame++) {
    const levels = levelsPerFrame[frame];
    const previous = levelsPerFrame[frame - 1];
    let rise = 0;
    for (let pitch = NOTE_LOW; pitch <= NOTE_HIGH; pitch++) {
      const gain = levels[pitch] - previous[pitch];
      if (gain > 0) rise += gain;
    }
    curve[frame] = rise;
  }
  return curve;
}

// detectBeat finds the pulse of the take: the lag between attacks that repeats
// best is the beat, and the offset of the strongest comb over that lag is where
// the beats fall. Without a clear pulse it falls back to the session tempo and
// to the start of the take, so an unrhythmic take is not swung at random.
function detectBeat(levelsPerFrame, hop, windowOffset, rate, fallbackBpm) {
  const fallbackBpmValue = Math.max(20, Number(fallbackBpm) || 120);
  const fallback = { beatFrames: (rate * 60) / fallbackBpmValue, phaseFrames: 0, tempo: 0 };
  if (levelsPerFrame.length < 4) return fallback;
  const curve = onsetStrength(levelsPerFrame);
  // Attacks are measured against the loudness of the take itself: the ripple a
  // held note leaves from frame to frame is a few percent of it, where an attack
  // is most of it, so only real attacks can seed the beat.
  let loudness = 0;
  for (const levels of levelsPerFrame) {
    let total = 0;
    for (let pitch = NOTE_LOW; pitch <= NOTE_HIGH; pitch++) total += levels[pitch];
    loudness += total;
  }
  const floor = (loudness / levelsPerFrame.length) * 0.12;
  if (!(floor > 0)) return fallback;
  const peaks = [];
  for (let frame = 1; frame < curve.length - 1; frame++) {
    if (curve[frame] < floor) continue;
    if (curve[frame] < curve[frame - 1] || curve[frame] < curve[frame + 1]) continue;
    peaks.push(frame);
  }
  if (peaks.length < MIN_ONSETS) return fallback;
  const lowLag = Math.max(2, Math.round((60 * rate) / (TEMPO_HIGH * hop)));
  const highLag = Math.min(curve.length - 1, Math.round((60 * rate) / (TEMPO_LOW * hop)));
  let beatLag = 0;
  let bestScore = 0;
  for (let lag = lowLag; lag <= highLag; lag++) {
    let sum = 0;
    let count = 0;
    for (let frame = 0; frame + lag < curve.length; frame++) {
      sum += curve[frame] * curve[frame + lag];
      count++;
    }
    if (!count) continue;
    // The prior keeps a fast tremolo from being read as the beat, and a slow
    // one from being read at half speed.
    const tempo = (60 * rate) / (lag * hop);
    const prior = Math.exp(-0.5 * (Math.log2(tempo / TEMPO_CENTER) / TEMPO_SPREAD) ** 2);
    const score = (sum / count) * prior;
    if (score > bestScore) {
      bestScore = score;
      beatLag = lag;
    }
  }
  if (!beatLag) return fallback;
  // The pulse has to explain a real share of the onset energy, otherwise a few
  // unrelated attacks would still be snapped to an invented tempo.
  let energy = 0;
  for (let frame = 0; frame < curve.length; frame++) energy += curve[frame] * curve[frame];
  if (bestScore * curve.length < energy * 0.2) return fallback;
  return refineBeat(curve, beatLag, hop, windowOffset, rate);
}

// beatFit matches the onset curve against a pulse of the given period. The
// magnitude says how well the period fits and the phase says where its beats
// fall, which is exactly the sine the curve is modelled with.
function beatFit(curve, period) {
  const omega = (2 * Math.PI) / period;
  let real = 0;
  let imag = 0;
  for (let frame = 0; frame < curve.length; frame++) {
    const weight = curve[frame];
    if (weight === 0) continue;
    real += weight * Math.cos(omega * frame);
    imag -= weight * Math.sin(omega * frame);
  }
  const offset = ((-Math.atan2(imag, real) / omega) % period + period) % period;
  return { score: Math.hypot(real, imag), offset };
}

// refineBeat searches the period between whole frames: a lag rounded to the
// frame grid is off by up to a few percent of the tempo, which a long take
// would drift out of phase with.
function refineBeat(curve, lag, hop, windowOffset, rate) {
  let bestPeriod = lag;
  let bestOffset = 0;
  let bestScore = -1;
  for (let period = Math.max(2, lag - 1); period <= lag + 1; period += 0.02) {
    const fit = beatFit(curve, period);
    if (fit.score <= bestScore) continue;
    bestScore = fit.score;
    bestPeriod = period;
    bestOffset = fit.offset;
  }
  const beatFrames = bestPeriod * hop;
  return { beatFrames, phaseFrames: bestOffset * hop + windowOffset, tempo: (60 * rate) / beatFrames };
}

// quantize snaps notes to the sixteenth grid of the detected beat and merges the
// copies the tracker produces when a note flickers, so the piano roll reads like
// played music.
function quantize(notes, step, phase) {
  const snap = (frame) => Math.max(0, Math.round((frame - phase) / step) * step + phase);
  const shortest = Math.max(1, Math.round(step));
  const merged = new Map();
  for (const note of notes) {
    const start = Math.round(snap(note.startFrame));
    const end = Math.max(start + shortest, Math.round(snap(note.endFrame)));
    const key = `${note.pitch}:${start}`;
    const existing = merged.get(key);
    if (!existing || existing.lengthFrames < end - start) {
      merged.set(key, { pitch: note.pitch, startFrame: start, lengthFrames: end - start, velocity: note.velocity });
    }
  }
  return [...merged.values()].sort((left, right) => left.startFrame - right.startFrame || left.pitch - right.pitch);
}

// melodyLine keeps the top note of each onset, which is the line a listener
// would hum along with: a chord is not turned into a part of its own.
function melodyLine(notes) {
  const groups = new Map();
  for (const note of notes) {
    const group = groups.get(note.startFrame);
    if (!group || note.pitch > group.pitch) groups.set(note.startFrame, { ...note });
  }
  const line = [...groups.values()].sort((left, right) => left.startFrame - right.startFrame);
  line.forEach((note, index) => {
    const next = line[index + 1];
    if (next && next.startFrame < note.startFrame + note.lengthFrames) note.lengthFrames = next.startFrame - note.startFrame;
  });
  return line;
}

// transcribeTake reads one take slice and returns its melody in frames of its
// own sample rate, relative to the slice start: one note per onset, the top line
// alone. It is snapped to the beat the take was played in, which comes back as
// "tempo" (0 when no pulse was found and the session tempo was used instead).
export function transcribeTake({ samples, channels = 1, sampleRate, bpm = 120, maxSeconds = MAX_SECONDS }) {
  if (!samples || !samples.length) throw new Error("this take has no audio to read");
  const rate = Math.max(8000, Math.round(Number(sampleRate) || 48000));
  const mono = toMono(samples, channels);
  const limit = Math.min(mono.length, Math.max(rate, Math.round((Number(maxSeconds) || MAX_SECONDS) * rate)));
  const hop = Math.max(128, Math.round(rate / 50));
  const window = hannWindow(WINDOW);
  const transform = createTransform(WINDOW);
  const real = new Float32Array(WINDOW);
  const imag = new Float32Array(WINDOW);
  const magnitudes = new Float32Array(WINDOW / 2);
  const voicesPerFrame = [];
  const levelsPerFrame = [];
  for (let at = 0; at + WINDOW <= limit; at += hop) {
    let energy = 0;
    for (let index = 0; index < WINDOW; index++) {
      const value = mono[at + index];
      energy += value * value;
      real[index] = value * window[index];
      imag[index] = 0;
    }
    if (Math.sqrt(energy / WINDOW) < MIN_RMS) {
      voicesPerFrame.push([]);
      levelsPerFrame.push(new Float32Array(NOTE_HIGH + 1));
      continue;
    }
    transform(real, imag);
    for (let bin = 0; bin < magnitudes.length; bin++) magnitudes[bin] = Math.hypot(real[bin], imag[bin]);
    const binHz = rate / WINDOW;
    const levels = pitchLevels(magnitudes, binHz);
    levelsPerFrame.push(levels);
    voicesPerFrame.push(frameVoices(levels, magnitudes, binHz));
  }
  const windowOffset = Math.round(WINDOW / 2);
  const beat = detectBeat(levelsPerFrame, hop, windowOffset, rate, bpm);
  const step = Math.max(1, beat.beatFrames / SUBDIVISION);
  // The grid phase+step*n is the same for any phase shifted by a step, so it
  // keeps the smallest one: a note cannot start before the take, and clamping
  // it there would move it by the whole beat instead of half a subdivision.
  const phase = ((beat.phaseFrames % step) + step) % step;
  const heard = quantize(trackNotes(voicesPerFrame, hop, windowOffset, MIN_FRAMES), step, phase);
  return {
    melody: melodyLine(heard),
    tempo: beat.tempo,
    beatFrames: beat.beatFrames,
    phaseFrames: phase,
    truncated: mono.length > limit,
    frames: limit,
  };
}
