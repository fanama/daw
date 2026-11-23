// Web Audio translation of the desktop chain, in the same fixed order:
// amp (drive + tone) -> cabinet -> five-band EQ -> compressor -> reverb.

function decibels(value) {
  return Math.pow(10, value / 20);
}

function clamp(value, min, max) {
  const number = Number(value);
  return Math.min(max, Math.max(min, Number.isFinite(number) ? number : 0));
}

function biquad(context, type, frequency, q, gainDB) {
  const filter = context.createBiquadFilter();
  filter.type = type;
  filter.frequency.value = Math.min(context.sampleRate * 0.49, Math.max(10, Number(frequency) || 1000));
  filter.Q.value = Math.max(0.0001, Number(q) || 0.7);
  filter.gain.value = Number.isFinite(gainDB) ? gainDB : 0;
  return filter;
}

function driveCurve(drive) {
  const size = 2048;
  const curve = new Float32Array(size);
  const amount = 1 + drive * 12;
  for (let index = 0; index < size; index++) {
    const input = (index / (size - 1)) * 2 - 1;
    curve[index] = Math.tanh(amount * input) / Math.tanh(amount);
  }
  return curve;
}

// Deterministic noise keeps an impulse identical across rebuilds, so changing
// another effect does not slightly alter the room. The tail is filtered more
// as it decays, which is what makes high damping sound like a soft room rather
// than simply a shorter one.
function seededRandom(seed) {
  let state = seed >>> 0;
  return () => {
    state = (state + 0x6d2b79f5) >>> 0;
    let value = state;
    value = Math.imul(value ^ (value >>> 15), value | 1);
    value ^= value + Math.imul(value ^ (value >>> 7), value | 61);
    return ((value ^ (value >>> 14)) >>> 0) / 4294967296;
  };
}

// reverbImpulseData builds two decorrelated channels: a short cluster of early
// reflections followed by a damped diffuse tail. The result is peak-normalised
// so every room size reaches the convolver at a comparable level.
export function reverbImpulseData(sampleRate, size, damp) {
  const rate = Math.max(8000, Number(sampleRate) || 48000);
  const room = clamp(size, 0, 1);
  const damping = clamp(damp, 0, 1);
  const length = Math.max(1, Math.round(rate * (0.4 + room * 2.6)));
  const channels = [new Float32Array(length), new Float32Array(length)];
  const roomScale = 0.7 + room * 1.1;
  const early = [
    [0.007, 0.38],
    [0.013, -0.26],
    [0.021, 0.2],
    [0.032, -0.14],
    [0.047, 0.09],
    [0.068, -0.06],
  ];
  const fadeLength = Math.max(1, Math.round(rate * 0.006));
  const coefficientBlock = 32;

  for (let channel = 0; channel < channels.length; channel++) {
    const data = channels[channel];
    const random = seededRandom(
      0x9e3779b9 ^ Math.round(room * 1000) ^ (Math.round(damping * 1000) << 10) ^ (channel * 0x85ebca6b),
    );
    for (let tap = 0; tap < early.length; tap++) {
      const [time, gain] = early[tap];
      const offset = Math.round((time * roomScale + channel * 0.0017 + tap * 0.0003) * rate);
      if (offset < length) data[offset] += gain;
    }

    let lowpassed = 0;
    let dcBlocked = 0;
    let coefficient = 0.5;
    for (let index = 0; index < length; index++) {
      if (index % coefficientBlock === 0) {
        const progress = index / length;
        const cutoff = 16000 * Math.pow(1 - progress, 0.35 + damping * 1.8) + 500;
        coefficient = 1 - Math.exp((-2 * Math.PI * cutoff) / rate);
      }
      const noise = random() * 2 - 1;
      lowpassed += coefficient * (noise - lowpassed);
      dcBlocked += 0.002 * (lowpassed - dcBlocked);
      const decay = Math.pow(1 - index / length, 1.4 + damping * 2.2);
      const fade = Math.min(1, index / fadeLength);
      data[index] += (lowpassed - dcBlocked) * decay * fade;
    }
  }

  let peak = 0;
  for (const data of channels) {
    for (let index = 0; index < length; index++) {
      const magnitude = Math.abs(data[index]);
      if (magnitude > peak) peak = magnitude;
    }
  }
  const scale = peak > 0 ? 0.9 / peak : 1;
  if (scale !== 1) {
    for (const data of channels) {
      for (let index = 0; index < length; index++) data[index] *= scale;
    }
  }
  return channels;
}

const REVERB_CACHE_LIMIT = 8;
const reverbCaches = new WeakMap();

// A convolver buffer depends only on the sample rate and the quantised size and
// damping, so tracks share it and dragging a slider does not rebuild the room.
function reverbImpulse(context, size, damp) {
  const room = Math.round(clamp(size, 0, 1) * 100);
  const damping = Math.round(clamp(damp, 0, 1) * 100);
  let cache = reverbCaches.get(context);
  if (!cache) {
    cache = new Map();
    reverbCaches.set(context, cache);
  }
  const key = `${room}:${damping}`;
  const hit = cache.get(key);
  if (hit) {
    cache.delete(key);
    cache.set(key, hit);
    return hit;
  }
  const channels = reverbImpulseData(context.sampleRate, room / 100, damping / 100);
  const buffer = context.createBuffer(channels.length, channels[0].length, context.sampleRate);
  channels.forEach((data, channel) => buffer.getChannelData(channel).set(data));
  cache.set(key, buffer);
  while (cache.size > REVERB_CACHE_LIMIT) cache.delete(cache.keys().next().value);
  return buffer;
}

// buildEffectsChain returns a node with one input and one output. Callers
// connect it into the monitor path and dispose it when the track changes.
export function buildEffectsChain(context, effects) {
  const input = context.createGain();
  const nodes = [input];
  let tail = input;

  const append = (node) => {
    tail.connect(node);
    tail = node;
    nodes.push(node);
    return node;
  };

  if (effects.Amp?.Enabled) {
    const amp = effects.Amp;
    const pre = context.createGain();
    pre.gain.value = 1 + amp.Drive * 9;
    append(pre);
    const shaper = context.createWaveShaper();
    shaper.curve = driveCurve(amp.Drive);
    shaper.oversample = "4x";
    append(shaper);
    append(biquad(context, "lowshelf", 120, 0.7, amp.BassDB));
    append(biquad(context, "peaking", 800, 0.9, amp.MidDB));
    append(biquad(context, "highshelf", 3200, 0.7, amp.TrebleDB));
    const level = context.createGain();
    level.gain.value = decibels(amp.LevelDB);
    append(level);
  }

  if (effects.Cab?.Enabled) {
    const character = effects.Cab.Character ?? 0.5;
    const highpass = biquad(context, "highpass", 85, 0.7, 0);
    highpass.frequency.value = 75 + character * 30;
    append(highpass);
    append(biquad(context, "peaking", 115, 1.1, character * 3));
    const lowpass = biquad(context, "lowpass", 5200 - character * 1800, 0.75, 0);
    append(lowpass);
  }

  if (effects.EQ?.Enabled) {
    for (const band of effects.EQ.Bands || []) {
      if (band.GainDB === 0) continue;
      append(biquad(context, "peaking", band.FrequencyHz, band.Q, band.GainDB));
    }
  }

  if (effects.Comp?.Enabled) {
    const comp = effects.Comp;
    const compressor = context.createDynamicsCompressor();
    compressor.threshold.value = comp.ThresholdDB;
    compressor.ratio.value = comp.Ratio;
    compressor.attack.value = comp.AttackMs / 1000;
    compressor.release.value = comp.ReleaseMs / 1000;
    compressor.knee.value = comp.KneeDB;
    append(compressor);
    if (comp.MakeupDB > 0) {
      const makeup = context.createGain();
      makeup.gain.value = decibels(comp.MakeupDB);
      append(makeup);
    }
  }

  if (effects.Reverb?.Enabled) {
    const reverb = effects.Reverb;
    const mix = clamp(reverb.Mix, 0, 1);
    const dry = context.createGain();
    const wet = context.createGain();
    // Equal-power crossfade keeps the perceived level steady across the mix
    // range instead of letting dry and wet add up past unity.
    dry.gain.value = Math.cos((mix * Math.PI) / 2);
    wet.gain.value = Math.sin((mix * Math.PI) / 2);
    const preDelay = context.createDelay(0.2);
    preDelay.delayTime.value = reverb.PreDelayMs / 1000;
    const convolver = context.createConvolver();
    convolver.buffer = reverbImpulse(context, reverb.Size, reverb.Damp);
    const output = context.createGain();
    tail.connect(dry);
    dry.connect(output);
    tail.connect(preDelay);
    preDelay.connect(convolver);
    convolver.connect(wet);
    wet.connect(output);
    nodes.push(dry, wet, preDelay, convolver, output);
    tail = output;
  }

  return {
    input,
    output: tail,
    nodes,
    dispose() {
      for (const node of nodes) {
        try {
          node.disconnect();
        } catch (error) {
          // A node may already be disconnected while rebuilding.
        }
      }
    },
  };
}
