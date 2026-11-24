// Effects presets are JSON files shaped like a track's chain:
// {"chain":[{"kind":"amp","amp":{...}}, ...]}. Presets that still list the
// five fixed blocks are accepted and merged into the current chain.

const BLOCKS = [
  {
    kind: "amp",
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
    kind: "cab",
    state: "Cab",
    fields: { enabled: "Enabled", character: "Character" },
  },
  {
    kind: "comp",
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
    kind: "reverb",
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

const BLOCK_BY_KIND = new Map(BLOCKS.map((block) => [block.kind, block]));

export function effectsToPreset(effects = {}) {
  const chain = Array.isArray(effects.Chain) ? effects.Chain : [];
  return { chain: chain.map(effectToPreset).filter(Boolean) };
}

export function effectsFromPreset(preset, current) {
  if (!isObject(preset)) throw new Error("That file is not a fanamix effects preset.");
  if (Array.isArray(preset.chain)) return chainFromPreset(preset.chain, current);
  if (BLOCKS.some((block) => block.kind in preset) || "eq" in preset) {
    return mergeLegacy(preset, current);
  }
  throw new Error("That file is not a fanamix effects preset.");
}

function effectToPreset(effect) {
  const kind = String(effect?.Kind || "").toLowerCase();
  const block = BLOCK_BY_KIND.get(kind);
  if (!block) {
    if (kind !== "eq" || !effect?.EQ) return null;
    const source = effect.EQ;
    return {
      kind,
      eq: {
        enabled: Boolean(source.Enabled),
        bands: (source.Bands || []).map((band) => ({
          frequencyHz: finiteOrZero(band.FrequencyHz),
          q: finiteOrZero(band.Q),
          gainDb: finiteOrZero(band.GainDB),
        })),
      },
    };
  }
  const source = effect[block.state] || {};
  const settings = {};
  for (const [key, stateKey] of Object.entries(block.fields)) {
    settings[key] = key === "enabled" ? Boolean(source[stateKey]) : finiteOrZero(source[stateKey]);
  }
  return { kind, [kind]: settings };
}

function chainFromPreset(entries, current) {
  if (!entries.length) throw new Error("The preset has no effects.");
  const chain = entries.map((entry, index) => {
    if (!isObject(entry)) throw new Error(`chain[${index}] must be an object.`);
    const kind = String(entry.kind || "").toLowerCase();
    if (kind === "eq") {
      if (!isObject(entry.eq)) throw new Error(`chain[${index}].eq must be an object.`);
      return { Kind: "eq", EQ: readEQ(entry.eq, baseSettings(current, "eq")) };
    }
    const block = BLOCK_BY_KIND.get(kind);
    if (!block) throw new Error(`chain[${index}] has an unknown effect kind.`);
    if (!isObject(entry[kind])) throw new Error(`chain[${index}].${kind} must be an object.`);
    return {
      Kind: kind,
      [block.state]: readBlock(block, entry[kind], baseSettings(current, kind)),
    };
  });
  return { Chain: chain };
}

function baseSettings(effects, kind) {
  const chain = Array.isArray(effects?.Chain) ? effects.Chain : [];
  const effect = chain.find((candidate) => String(candidate?.Kind || "").toLowerCase() === kind);
  if (!effect) return {};
  const state = kind === "eq" ? "EQ" : BLOCK_BY_KIND.get(kind).state;
  return effect[state] || {};
}

function readBlock(block, source, base) {
  const settings = { ...base };
  for (const [key, stateKey] of Object.entries(block.fields)) {
    if (!(key in source)) continue;
    if (key === "enabled") {
      if (typeof source[key] !== "boolean") throw new Error(`${block.kind}.enabled must be true or false.`);
      settings[stateKey] = source[key];
    } else {
      settings[stateKey] = finite(source[key], `${block.kind}.${key}`);
    }
  }
  return settings;
}

function readEQ(source, base) {
  const settings = { ...base };
  // The equalizer is always five bands, whatever a partial chain claimed.
  settings.Bands = Array.isArray(settings.Bands) && settings.Bands.length === 5
    ? settings.Bands.map((band) => ({ ...band }))
    : Array.from({ length: 5 }, () => ({ FrequencyHz: 0, Q: 0, GainDB: 0 }));
  if ("enabled" in source) {
    if (typeof source.enabled !== "boolean") throw new Error("eq.enabled must be true or false.");
    settings.Enabled = source.enabled;
  }
  if ("bands" in source) {
    if (!Array.isArray(source.bands) || source.bands.length !== 5) {
      throw new Error("eq.bands must list 5 bands.");
    }
    settings.Bands = source.bands.map((band, index) => {
      if (!isObject(band)) throw new Error(`eq.bands[${index}] must be an object.`);
      const next = { ...settings.Bands[index] };
      if ("frequencyHz" in band) next.FrequencyHz = finite(band.frequencyHz, `eq.bands[${index}].frequencyHz`);
      if ("q" in band) next.Q = finite(band.q, `eq.bands[${index}].q`);
      if ("gainDb" in band) next.GainDB = finite(band.gainDb, `eq.bands[${index}].gainDb`);
      return next;
    });
  }
  return settings;
}

function mergeLegacy(preset, current) {
  const effects = JSON.parse(JSON.stringify(current || {}));
  effects.Chain = Array.isArray(effects.Chain) ? effects.Chain : [];
  let changed = false;
  for (const block of BLOCKS) {
    if (!(block.kind in preset)) continue;
    const source = preset[block.kind];
    if (!isObject(source)) throw new Error(`${block.kind} must be an object.`);
    const effect = ensureKind(effects.Chain, block.kind, block.state);
    effect[block.state] = readBlock(block, source, effect[block.state] || {});
    changed = true;
  }
  if ("eq" in preset) {
    const source = preset.eq;
    if (!isObject(source)) throw new Error("eq must be an object.");
    const effect = ensureKind(effects.Chain, "eq", "EQ");
    effect.EQ = readEQ(source, effect.EQ || {});
    changed = true;
  }
  if (!changed) throw new Error("That JSON file contains no effect settings.");
  return effects;
}

function ensureKind(chain, kind, state) {
  let effect = chain.find((candidate) => String(candidate?.Kind || "").toLowerCase() === kind);
  if (!effect) {
    effect = { Kind: kind, [state]: {} };
    chain.push(effect);
  }
  return effect;
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
