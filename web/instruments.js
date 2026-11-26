// Soft-synth voices for instrument tracks. Each note is scheduled on a Web
// Audio context, so the same code feeds live playback and the offline mixdown.

export const INSTRUMENTS = [
  { id: "piano", label: "Piano" },
  { id: "bass", label: "Bass" },
  { id: "guitar", label: "Guitar" },
  { id: "drums", label: "Drums" },
];

// DRUM_KIT maps the General MIDI drum notes to sequencer rows. The order is the
// kit order used by drum machines: low, loud pieces first.
export const DRUM_KIT = [
  { pitch: 36, label: "Kick" },
  { pitch: 38, label: "Snare" },
  { pitch: 39, label: "Clap" },
  { pitch: 42, label: "Closed hat" },
  { pitch: 46, label: "Open hat" },
  { pitch: 45, label: "Low tom" },
  { pitch: 47, label: "Mid tom" },
  { pitch: 50, label: "High tom" },
  { pitch: 49, label: "Crash" },
  { pitch: 51, label: "Ride" },
];

export function midiFrequency(pitch) {
  return 440 * Math.pow(2, (Number(pitch) - 69) / 12);
}

// scheduleInstrumentNote plays one note into destination and, when given, taps
// the same signal into an analyser. Nodes are appended to nodes so the
// transport can stop them on the next seek or stop.
export function scheduleInstrumentNote(context, destination, instrument, pitch, when, duration, velocity, nodes = [], tap = null) {
  const frequency = midiFrequency(pitch);
  if (!Number.isFinite(frequency) || frequency <= 0) return;
  const level = Math.min(1, Math.max(0.02, (Number(velocity) || 100) / 127));
  const hold = Math.max(0.05, Number(duration) || 0.25);
  const connect = (node) => {
    node.connect(destination);
    if (tap) node.connect(tap);
  };
  const start = Math.max(0, Number(when) || 0);
  switch (instrument) {
    case "bass":
      bassNote(context, connect, frequency, start, hold, level, nodes);
      return;
    case "guitar":
      guitarNote(context, connect, frequency, start, hold, level, nodes);
      return;
    case "drums":
      drumNote(context, connect, pitch, start, level, nodes);
      return;
    default:
      pianoNote(context, connect, frequency, start, hold, level, nodes);
  }
}

// pianoNote layers decaying sine partials: a quick attack, a bright onset and
// a long tail, which reads as a piano without a sample library.
function pianoNote(context, connect, frequency, when, hold, level, nodes) {
  const output = context.createGain();
  output.gain.value = level * 0.5;
  connect(output);
  const peak = Math.max(when + 0.05, when + hold);
  const tail = Math.min(when + 3.2, Math.max(peak + 0.05, when + 0.8));
  for (const [ratio, amplitude] of [[1, 0.6], [2, 0.22], [3, 0.12], [4, 0.06]]) {
    if (frequency * ratio > 14000) continue;
    const oscillator = context.createOscillator();
    oscillator.type = "sine";
    oscillator.frequency.value = frequency * ratio;
    const gain = context.createGain();
    gain.gain.setValueAtTime(0.0001, when);
    gain.gain.linearRampToValueAtTime(amplitude * level, when + 0.008);
    gain.gain.exponentialRampToValueAtTime(Math.max(0.0002, amplitude * level * 0.42), when + 0.18);
    gain.gain.exponentialRampToValueAtTime(0.0001, tail);
    oscillator.connect(gain);
    gain.connect(output);
    oscillator.start(when);
    oscillator.stop(tail + 0.05);
    nodes.push(oscillator, gain);
  }
  nodes.push(output);
}

// bassNote mixes a triangle fundamental with a sine sub through a low-pass;
// the short filter envelope keeps the attack audible and the rest round.
function bassNote(context, connect, frequency, when, hold, level, nodes) {
  const output = context.createGain();
  output.gain.value = level * 0.7;
  connect(output);
  const filter = context.createBiquadFilter();
  filter.type = "lowpass";
  filter.frequency.value = Math.min(2200, frequency * 9 + 320);
  filter.Q.value = 0.9;
  const envelope = context.createGain();
  const end = when + hold;
  envelope.gain.setValueAtTime(0.0001, when);
  envelope.gain.linearRampToValueAtTime(level, when + 0.012);
  envelope.gain.setTargetAtTime(level * 0.55, when + 0.06, 0.15);
  envelope.gain.setTargetAtTime(0.0001, Math.max(when + 0.09, end - 0.02), 0.035);
  const oscillator = context.createOscillator();
  oscillator.type = "triangle";
  oscillator.frequency.value = frequency;
  const sub = context.createOscillator();
  sub.type = "sine";
  sub.frequency.value = frequency;
  const subGain = context.createGain();
  subGain.gain.value = 0.55;
  oscillator.connect(filter);
  sub.connect(subGain);
  subGain.connect(filter);
  filter.connect(envelope);
  envelope.connect(output);
  oscillator.start(when);
  sub.start(when);
  const stop = end + 0.25;
  oscillator.stop(stop);
  sub.stop(stop);
  nodes.push(oscillator, sub, subGain, filter, envelope, output);
}

// guitarRing builds a Karplus-Strong string: a noise burst circulating through
// a short delay line, low-passed a little more on every pass.
function guitarRing(context, frequency, hold) {
  const rate = context.sampleRate;
  const seconds = Math.min(4.5, Math.max(0.35, hold + 0.7));
  const frames = Math.max(1, Math.floor(rate * seconds));
  const buffer = context.createBuffer(1, frames, rate);
  const data = buffer.getChannelData(0);
  const period = Math.max(2, Math.round(rate / frequency));
  const ring = new Float32Array(period);
  for (let index = 0; index < period; index++) ring[index] = Math.random() * 2 - 1;
  const decay = frequency < 90 ? 0.992 : frequency < 240 ? 0.995 : 0.997;
  let index = 0;
  let previous = 0;
  for (let frame = 0; frame < frames; frame++) {
    const current = ring[index];
    data[frame] = current;
    ring[index] = (current + previous) * 0.5 * decay;
    previous = current;
    index = (index + 1) % period;
  }
  const fade = Math.max(1, Math.floor(rate * 0.002));
  for (let frame = 0; frame < fade; frame++) data[frame] *= frame / fade;
  return buffer;
}

// Drum voices follow the General MIDI note numbers in DRUM_KIT. Skins are
// pitched sines with a fast drop; metal and claps are filtered noise bursts.
function drumNote(context, connect, pitch, when, level, nodes) {
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

function guitarNote(context, connect, frequency, when, hold, level, nodes) {
  const source = context.createBufferSource();
  source.buffer = guitarRing(context, frequency, hold);
  const body = context.createBiquadFilter();
  body.type = "lowpass";
  body.frequency.value = Math.min(7000, frequency * 12 + 1500);
  body.Q.value = 0.8;
  const output = context.createGain();
  const end = when + hold;
  output.gain.setValueAtTime(level * 0.6, when);
  output.gain.setValueAtTime(level * 0.6, Math.max(when, end - 0.03));
  output.gain.linearRampToValueAtTime(0.0001, end + 0.12);
  connect(output);
  source.connect(body);
  body.connect(output);
  source.start(when);
  source.stop(Math.min(when + source.buffer.duration, end + 0.15));
  nodes.push(source, body, output);
}
