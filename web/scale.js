// Key detection for a written part: the pitch classes of its notes, weighted by
// how long each one sounds, are matched against the major and minor key profiles
// of Krumhansl and Schmuckler. The best match names the scale, and the degrees
// that carry it (tonic, third, dominant) come back with it. It is pure, so the
// piano roll can ask for it on every paint.

const NOTE_NAMES = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"];
// How much a pitch class stands out in each mode, read from listeners' ratings
// of probe tones against a key: the tonic is highest, then the dominant.
const MAJOR_PROFILE = [6.35, 2.23, 3.48, 2.33, 4.38, 4.09, 2.52, 5.19, 2.39, 3.66, 2.29, 2.88];
const MINOR_PROFILE = [6.33, 2.68, 3.52, 5.38, 2.6, 3.53, 2.54, 4.75, 3.98, 2.69, 3.34, 3.17];
const SCALE_STEPS = { major: [0, 2, 4, 5, 7, 9, 11], minor: [0, 2, 3, 5, 7, 8, 10] };
const THIRD = 2; // index of the third in the scale steps
const FIFTH = 4;
const MIN_CLASSES = 3; // fewer sounding pitch classes name no key

// weightedHistogram scores each pitch class by the square root of the time its
// notes last, so a held note matters more than a passing one without letting a
// single long note decide the key on its own.
function weightedHistogram(notes) {
  const histogram = new Float64Array(12);
  for (const note of notes || []) {
    const pitch = Math.round(Number(note.pitch));
    if (!Number.isFinite(pitch)) continue;
    const length = Math.max(1, Number(note.lengthFrames) || 1);
    histogram[((pitch % 12) + 12) % 12] += Math.sqrt(length);
  }
  return histogram;
}

// correlation is Pearson's, so the profile's own scale of values does not matter
// and a key is chosen on the shape of the part, not on how loud it was.
function correlation(histogram, profile, tonic) {
  const values = new Float64Array(12);
  let meanHistogram = 0;
  let meanProfile = 0;
  for (let index = 0; index < 12; index++) {
    values[index] = profile[(index - tonic + 12) % 12];
    meanHistogram += histogram[index] / 12;
    meanProfile += values[index] / 12;
  }
  let covariance = 0;
  let varianceHistogram = 0;
  let varianceProfile = 0;
  for (let index = 0; index < 12; index++) {
    const histogramSide = histogram[index] - meanHistogram;
    const profileSide = values[index] - meanProfile;
    covariance += histogramSide * profileSide;
    varianceHistogram += histogramSide * histogramSide;
    varianceProfile += profileSide * profileSide;
  }
  if (!varianceHistogram || !varianceProfile) return 0;
  return covariance / Math.sqrt(varianceHistogram * varianceProfile);
}

// detectKey returns the scale of a part as { tonic, mode, name, scale, tonicName,
// thirdName, fifthName }, or null when the part is too thin to name a key.
export function detectKey(notes) {
  const histogram = weightedHistogram(notes);
  let total = 0;
  let sounding = 0;
  for (const value of histogram) {
    total += value;
    if (value > 0) sounding++;
  }
  if (sounding < MIN_CLASSES || !(total > 0)) return null;
  let best = { score: -Infinity, tonic: 0, mode: "major" };
  for (let tonic = 0; tonic < 12; tonic++) {
    for (const mode of ["major", "minor"]) {
      const profile = mode === "major" ? MAJOR_PROFILE : MINOR_PROFILE;
      const score = correlation(histogram, profile, tonic);
      if (score <= best.score) continue;
      best = { score, tonic, mode };
    }
  }
  const steps = SCALE_STEPS[best.mode];
  const nameAt = (semitone) => NOTE_NAMES[(best.tonic + semitone) % 12];
  return {
    tonic: best.tonic,
    mode: best.mode,
    name: `${nameAt(0)} ${best.mode}`,
    scale: steps.map(nameAt),
    tonicName: nameAt(0),
    thirdName: nameAt(steps[THIRD]),
    fifthName: nameAt(steps[FIFTH]),
  };
}
