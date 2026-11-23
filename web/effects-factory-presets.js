// Built-in monitor-effect presets. Each entry is a complete chain in the same
// camelCase shape as a downloaded preset, so choosing one replaces the track's
// current settings instead of layering over them.

// The five EQ bands sit at musical centers rather than the 20 Hz / 20 kHz
// extremes: weight, body, midrange, presence and air.
const EQ_FREQUENCIES = [60, 250, 1000, 4000, 12000];
const EQ_Q = [0.7, 0.8, 1, 0.8, 0.7];

function eqBands(gains) {
  return EQ_FREQUENCIES.map((frequencyHz, index) => ({
    frequencyHz,
    q: EQ_Q[index],
    gainDb: gains[index],
  }));
}

const AMP_OFF = { enabled: false, drive: 0, bassDb: 0, midDb: 0, trebleDb: 0, levelDb: -6 };

export const FACTORY_PRESETS = [
  {
    name: "Electric — Clean",
    group: "Guitar",
    preset: {
      amp: { enabled: true, drive: 0.18, bassDb: 1.5, midDb: -0.5, trebleDb: 2, levelDb: -4 },
      cab: { enabled: true, character: 0.55 },
      eq: { enabled: true, bands: eqBands([-4, 1, 1.5, 2, -2]) },
      comp: { enabled: true, thresholdDb: -20, ratio: 2.5, attackMs: 15, releaseMs: 150, kneeDb: 8, makeupDb: 2 },
      reverb: { enabled: true, size: 0.35, damp: 0.55, mix: 0.15, preDelayMs: 12 },
    },
  },
  {
    name: "Electric — Crunch",
    group: "Guitar",
    preset: {
      amp: { enabled: true, drive: 0.5, bassDb: 1, midDb: 2, trebleDb: 1, levelDb: -8 },
      cab: { enabled: true, character: 0.7 },
      eq: { enabled: true, bands: eqBands([-5, 1, 2.5, 1, -3]) },
      comp: { enabled: true, thresholdDb: -18, ratio: 3, attackMs: 10, releaseMs: 130, kneeDb: 6, makeupDb: 2 },
      reverb: { enabled: true, size: 0.3, damp: 0.6, mix: 0.12, preDelayMs: 8 },
    },
  },
  {
    name: "Electric — Lead",
    group: "Guitar",
    preset: {
      amp: { enabled: true, drive: 0.72, bassDb: 0, midDb: 3, trebleDb: 1.5, levelDb: -11 },
      cab: { enabled: true, character: 0.8 },
      eq: { enabled: true, bands: eqBands([-6, 0, 3, 2, -4]) },
      comp: { enabled: true, thresholdDb: -20, ratio: 4, attackMs: 6, releaseMs: 110, kneeDb: 4, makeupDb: 3 },
      reverb: { enabled: true, size: 0.45, damp: 0.45, mix: 0.2, preDelayMs: 22 },
    },
  },
  {
    name: "Bass — DI",
    group: "Guitar",
    preset: {
      amp: { enabled: true, drive: 0.12, bassDb: 3, midDb: 0.5, trebleDb: -1, levelDb: -5 },
      cab: { enabled: true, character: 0.4 },
      eq: { enabled: true, bands: eqBands([-4, 3.5, 1.5, -1, -6]) },
      comp: { enabled: true, thresholdDb: -20, ratio: 4, attackMs: 12, releaseMs: 110, kneeDb: 6, makeupDb: 3 },
      reverb: { enabled: false, size: 0.2, damp: 0.7, mix: 0.05, preDelayMs: 5 },
    },
  },
  {
    name: "Acoustic — Steel",
    group: "Guitar",
    preset: {
      amp: { ...AMP_OFF },
      cab: { enabled: false, character: 0.5 },
      eq: { enabled: true, bands: eqBands([-4, 2, 0.5, 2, -3]) },
      comp: { enabled: true, thresholdDb: -22, ratio: 2, attackMs: 18, releaseMs: 170, kneeDb: 10, makeupDb: 2.5 },
      reverb: { enabled: true, size: 0.3, damp: 0.6, mix: 0.16, preDelayMs: 15 },
    },
  },
  {
    name: "Acoustic — Nylon",
    group: "Guitar",
    preset: {
      amp: { ...AMP_OFF },
      cab: { enabled: false, character: 0.5 },
      eq: { enabled: true, bands: eqBands([-5, 2.5, 1, 0.5, -4]) },
      comp: { enabled: true, thresholdDb: -24, ratio: 1.8, attackMs: 22, releaseMs: 190, kneeDb: 12, makeupDb: 2 },
      reverb: { enabled: true, size: 0.4, damp: 0.55, mix: 0.2, preDelayMs: 18 },
    },
  },
  {
    name: "Lead Vocal",
    group: "Voice",
    preset: {
      amp: { ...AMP_OFF },
      cab: { enabled: false, character: 0.5 },
      eq: { enabled: true, bands: eqBands([-8, -2.5, 1.5, 2, -3]) },
      comp: { enabled: true, thresholdDb: -24, ratio: 3, attackMs: 8, releaseMs: 110, kneeDb: 6, makeupDb: 4.5 },
      reverb: { enabled: true, size: 0.4, damp: 0.5, mix: 0.2, preDelayMs: 25 },
    },
  },
  {
    name: "Backing Vocal",
    group: "Voice",
    preset: {
      amp: { ...AMP_OFF },
      cab: { enabled: false, character: 0.5 },
      eq: { enabled: true, bands: eqBands([-8, -3, 1, 1.5, -4]) },
      comp: { enabled: true, thresholdDb: -26, ratio: 3.5, attackMs: 6, releaseMs: 100, kneeDb: 6, makeupDb: 4.5 },
      reverb: { enabled: true, size: 0.5, damp: 0.45, mix: 0.28, preDelayMs: 28 },
    },
  },
  {
    name: "Spoken Word",
    group: "Voice",
    preset: {
      amp: { ...AMP_OFF },
      cab: { enabled: false, character: 0.5 },
      eq: { enabled: true, bands: eqBands([-10, -3, 1.5, 1.5, -4]) },
      comp: { enabled: true, thresholdDb: -22, ratio: 2.5, attackMs: 3, releaseMs: 90, kneeDb: 8, makeupDb: 4 },
      reverb: { enabled: true, size: 0.18, damp: 0.65, mix: 0.06, preDelayMs: 8 },
    },
  },
];
