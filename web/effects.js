// Web Audio translation of the desktop chain, in the same fixed order:
// amp (drive + tone) -> cabinet -> 3-band EQ -> compressor -> reverb.

function decibels(value) {
  return Math.pow(10, value / 20);
}

function biquad(context, type, frequency, q, gainDB) {
  const filter = context.createBiquadFilter();
  filter.type = type;
  filter.frequency.value = frequency;
  filter.Q.value = q;
  filter.gain.value = gainDB;
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

function reverbImpulse(context, size, damp) {
  const seconds = 0.35 + size * 2.6;
  const length = Math.max(1, Math.floor(context.sampleRate * seconds));
  const impulse = context.createBuffer(2, length, context.sampleRate);
  for (let channel = 0; channel < 2; channel++) {
    const data = impulse.getChannelData(channel);
    let filtered = 0;
    for (let index = 0; index < length; index++) {
      const decay = Math.pow(1 - index / length, 1 + damp * 3);
      const noise = (Math.random() * 2 - 1) * decay;
      filtered = filtered * 0.4 + noise * 0.6;
      data[index] = filtered;
    }
  }
  return impulse;
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
    const dry = context.createGain();
    dry.gain.value = 1;
    const wet = context.createGain();
    wet.gain.value = reverb.Mix;
    dry.gain.value = Math.max(0.2, 1 - reverb.Mix * 0.3);
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
