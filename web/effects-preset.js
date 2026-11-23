// Effects presets are JSON files shaped like a project's "effects" block:
// camelCase keys and the five fixed blocks. Downloads and loads stay local to
// the page; the Go core normalizes the chain after a load.

const BLOCKS = [
  {
    key: "amp",
    state: "Amp",
    fields: {
      enabled: "Enabled",
      drive: "Drive",
      bassDb: "BassDB",
      midDb: "MidDB",
      trebleDb: "TrebleDB",
      levelDb: "LevelDB",
    },
  },
  {
    key: "cab",
    state: "Cab",
    fields: { enabled: "Enabled", character: "Character" },
  },
  {
    key: "comp",
    state: "Comp",
    fields: {
      enabled: "Enabled",
      thresholdDb: "ThresholdDB",
      ratio: "Ratio",
      attackMs: "AttackMs",
      releaseMs: "ReleaseMs",
      kneeDb: "KneeDB",
      makeupDb: "MakeupDB",
    },
  },
  {
    key: "reverb",
    state: "Reverb",
    fields: {
      enabled: "Enabled",
      size: "Size",
      damp: "Damp",
      mix: "Mix",
      preDelayMs: "PreDelayMs",
    },
  },
];

export function effectsToPreset(effects = {}) {
  const preset = {
    amp: {},
    cab: {},
    eq: {
      enabled: Boolean(effects.EQ?.Enabled),
      bands: (effects.EQ?.Bands || []).map((band) => ({
        frequencyHz: finiteOrZero(band.FrequencyHz),
        q: finiteOrZero(band.Q),
        gainDb: finiteOrZero(band.GainDB),
      })),
    },
    comp: {},
    reverb: {},
  };
  for (const block of BLOCKS) {
    const source = effects[block.state] || {};
    for (const [key, stateKey] of Object.entries(block.fields)) {
      preset[block.key][key] = key === "enabled" ? Boolean(source[stateKey]) : finiteOrZero(source[stateKey]);
    }
  }
  return preset;
}

export function effectsFromPreset(preset, current) {
  if (!isObject(preset)) throw new Error("That file is not a gorecord effects preset.");
  const effects = JSON.parse(JSON.stringify(current));
  let changed = false;

  for (const block of BLOCKS) {
    if (!(block.key in preset)) continue;
    const source = preset[block.key];
    if (!isObject(source)) throw new Error(`${block.key} must be an object.`);
    const target = effects[block.state];
    for (const [key, stateKey] of Object.entries(block.fields)) {
      if (!(key in source)) continue;
      if (key === "enabled") {
        if (typeof source[key] !== "boolean") throw new Error(`${block.key}.enabled must be true or false.`);
        target[stateKey] = source[key];
      } else {
        target[stateKey] = finite(source[key], `${block.key}.${key}`);
      }
      changed = true;
    }
  }

  if ("eq" in preset) {
    const source = preset.eq;
    if (!isObject(source)) throw new Error("eq must be an object.");
    if ("enabled" in source) {
      if (typeof source.enabled !== "boolean") throw new Error("eq.enabled must be true or false.");
      effects.EQ.Enabled = source.enabled;
      changed = true;
    }
    if ("bands" in source) {
      const bands = effects.EQ.Bands;
      if (!Array.isArray(source.bands) || source.bands.length !== bands.length) {
        throw new Error(`eq.bands must list ${bands.length} bands.`);
      }
      effects.EQ.Bands = source.bands.map((band, index) => {
        if (!isObject(band)) throw new Error(`eq.bands[${index}] must be an object.`);
        const next = { ...bands[index] };
        if ("frequencyHz" in band) next.FrequencyHz = finite(band.frequencyHz, `eq.bands[${index}].frequencyHz`);
        if ("q" in band) next.Q = finite(band.q, `eq.bands[${index}].q`);
        if ("gainDb" in band) next.GainDB = finite(band.gainDb, `eq.bands[${index}].gainDb`);
        return next;
      });
      changed = true;
    }
  }

  if (!changed) throw new Error("That JSON file contains no effect settings.");
  return effects;
}

function isObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function finite(value, path) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw new Error(`${path} must be a number.`);
  }
  return value;
}

function finiteOrZero(value) {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}
