// Synthesized drum kit voices, dispatched by General MIDI note number.
// Each hit is short and self-contained so playback and mixdown share it.

// Skins are pitched sines with a fast drop; metal and claps are filtered noise
export function drumNote(context, connect, pitch, when, level, nodes) {
  switch (pitch) {
    case 35:
    case 36:
      return kickDrum(context, connect, when, level, nodes);
    case 38:
    case 40:
      return snareDrum(context, connect, when, level, nodes);
    case 39:
      return clapDrum(context, connect, when, level, nodes);
    case 42:
      return hatDrum(context, connect, when, level, 0.05, nodes);
    case 44:
      return hatDrum(context, connect, when, level, 0.09, nodes);
    case 46:
      return hatDrum(context, connect, when, level, 0.42, nodes);
    case 41:
    case 43:
    case 45:
      return tomDrum(context, connect, when, 110, level, nodes);
    case 47:
    case 48:
      return tomDrum(context, connect, when, 150, level, nodes);
    case 50:
      return tomDrum(context, connect, when, 190, level, nodes);
    case 49:
    case 57:
      return cymbalDrum(context, connect, when, level, 1.1, 5000, nodes);
    case 51:
    case 59:
      return cymbalDrum(context, connect, when, level * 0.8, 0.7, 6500, nodes);
    default:
      return tomDrum(context, connect, when, 120 * Math.pow(2, (pitch - 45) / 12), level * 0.8, nodes);
  }
}

function noiseSource(context, seconds) {
  const frames = Math.max(1, Math.floor(context.sampleRate * seconds));
  const buffer = context.createBuffer(1, frames, context.sampleRate);
  const data = buffer.getChannelData(0);
  for (let index = 0; index < frames; index++) data[index] = Math.random() * 2 - 1;
  const source = context.createBufferSource();
  source.buffer = buffer;
  return source;
}

function kickDrum(context, connect, when, level, nodes) {
  const oscillator = context.createOscillator();
  oscillator.type = "sine";
  oscillator.frequency.setValueAtTime(150, when);
  oscillator.frequency.exponentialRampToValueAtTime(46, when + 0.12);
  const gain = context.createGain();
  gain.gain.setValueAtTime(level, when);
  gain.gain.exponentialRampToValueAtTime(0.0001, when + 0.36);
  oscillator.connect(gain);
  connect(gain);
  oscillator.start(when);
  oscillator.stop(when + 0.4);
  nodes.push(oscillator, gain);
}

function snareDrum(context, connect, when, level, nodes) {
  const noise = noiseSource(context, 0.24);
  const filter = context.createBiquadFilter();
  filter.type = "highpass";
  filter.frequency.value = 1400;
  const gain = context.createGain();
  gain.gain.setValueAtTime(level * 0.8, when);
  gain.gain.exponentialRampToValueAtTime(0.0001, when + 0.2);
  noise.connect(filter);
  filter.connect(gain);
  connect(gain);
  noise.start(when);
  noise.stop(when + 0.22);
  const body = context.createOscillator();
  body.type = "triangle";
  body.frequency.value = 185;
  const bodyGain = context.createGain();
  bodyGain.gain.setValueAtTime(level * 0.45, when);
  bodyGain.gain.exponentialRampToValueAtTime(0.0001, when + 0.1);
  body.connect(bodyGain);
  connect(bodyGain);
  body.start(when);
  body.stop(when + 0.12);
  nodes.push(noise, filter, gain, body, bodyGain);
}

function clapDrum(context, connect, when, level, nodes) {
  const noise = noiseSource(context, 0.22);
  const filter = context.createBiquadFilter();
  filter.type = "bandpass";
  filter.frequency.value = 1150;
  filter.Q.value = 0.8;
  const gain = context.createGain();
  gain.gain.setValueAtTime(0.0001, when);
  for (let burst = 0; burst < 3; burst++) {
    const at = when + burst * 0.012;
    gain.gain.setValueAtTime(level * 0.7, at);
    gain.gain.exponentialRampToValueAtTime(level * 0.15, at + 0.008);
  }
  gain.gain.setValueAtTime(level * 0.55, when + 0.036);
  gain.gain.exponentialRampToValueAtTime(0.0001, when + 0.18);
  noise.connect(filter);
  filter.connect(gain);
  connect(gain);
  noise.start(when);
  noise.stop(when + 0.2);
  nodes.push(noise, filter, gain);
}

function hatDrum(context, connect, when, level, decay, nodes) {
  const noise = noiseSource(context, decay + 0.05);
  const filter = context.createBiquadFilter();
  filter.type = "highpass";
  filter.frequency.value = 7200;
  const gain = context.createGain();
  gain.gain.setValueAtTime(level * 0.5, when);
  gain.gain.exponentialRampToValueAtTime(0.0001, when + decay);
  noise.connect(filter);
  filter.connect(gain);
  connect(gain);
  noise.start(when);
  noise.stop(when + decay + 0.02);
  nodes.push(noise, filter, gain);
}

function tomDrum(context, connect, when, frequency, level, nodes) {
  const oscillator = context.createOscillator();
  oscillator.type = "sine";
  oscillator.frequency.setValueAtTime(frequency, when);
  oscillator.frequency.exponentialRampToValueAtTime(frequency * 0.62, when + 0.12);
  const gain = context.createGain();
  gain.gain.setValueAtTime(level * 0.8, when);
  gain.gain.exponentialRampToValueAtTime(0.0001, when + 0.3);
  oscillator.connect(gain);
  connect(gain);
  oscillator.start(when);
  oscillator.stop(when + 0.32);
  nodes.push(oscillator, gain);
}

function cymbalDrum(context, connect, when, level, decay, cutoff, nodes) {
  const noise = noiseSource(context, decay + 0.05);
  const filter = context.createBiquadFilter();
  filter.type = "highpass";
  filter.frequency.value = cutoff;
  const gain = context.createGain();
  gain.gain.setValueAtTime(level * 0.45, when);
  gain.gain.exponentialRampToValueAtTime(0.0001, when + decay);
  noise.connect(filter);
  filter.connect(gain);
  connect(gain);
  noise.start(when);
  noise.stop(when + decay + 0.02);
  nodes.push(noise, filter, gain);
}
