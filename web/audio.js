// Browser audio engine. Capture and monitoring run through the Web Audio API;
// take audio is handed back to the Go core for persistence and export.

import { buildEffectsChain } from "./effects.js";
import { detectPitch, noteReading } from "./tuner.js";
import { audibleTakes } from "./model.js";

const BLOCK_FRAMES = 128;

function resample(samples, channels, fromRate, toRate) {
  if (fromRate === toRate) return samples;
  const frames = Math.floor(samples.length / channels);
  if (frames === 0) return samples;
  const targetFrames = Math.max(1, Math.round((frames * toRate) / fromRate));
  const output = new Float32Array(targetFrames * channels);
  const step = fromRate / toRate;
  for (let frame = 0; frame < targetFrames; frame++) {
    const position = Math.min(frames - 1, frame * step);
    const index = Math.floor(position);
    const next = Math.min(frames - 1, index + 1);
    const mix = position - index;
    for (let channel = 0; channel < channels; channel++) {
      const first = samples[index * channels + channel];
      const second = samples[next * channels + channel];
      output[frame * channels + channel] = first + (second - first) * mix;
    }
  }
  return output;
}

function trackLevel(track) {
  return Math.pow(10, (Number(track.volumeDb) || 0) / 20);
}

function trackPan(track) {
  return Math.max(-1, Math.min(1, Number(track.pan) || 0));
}

export class AudioEngine {
  constructor() {
    this.ctx = null;
    this.sessionRate = 48000;
    this.master = null;
    this.monitor = null;
    this.monitorEnabled = true;
    this.masterLevel = 1;
    this.stream = null;
    this.input = null;
    this.splitter = null;
    this.system = false;
    this.chains = new Map();
    this.playbackChains = new Map();
    this.playbackVolumes = new Map();
    this.playbackAnalysers = new Map();
    this.tuners = null;
    this.sources = [];
    this.takeGains = new Map();
    this.buffers = new Map();
    this.anchorTime = 0;
    this.anchorFrame = 0;
    this.playing = false;
    this.idleFrame = 0;
    this.latencyFrames = 0;
    this.capture = null;
    this.opening = null;
    this.click = { Enabled: false, BPM: 120, BeatsPerBar: 4 };
    this.clickTimer = 0;
    this.lastState = null;
  }

  async ensureContext(sampleRate) {
    if (this.ctx) {
      if (this.ctx.state === "suspended") await this.ctx.resume();
      return;
    }
    this.sessionRate = sampleRate;
    this.ctx = new AudioContext({ sampleRate, latencyHint: "interactive" });
    await this.ctx.audioWorklet.addModule("capture-worklet.js");
    this.master = this.ctx.createGain();
    this.master.gain.value = this.masterLevel;
    this.master.connect(this.ctx.destination);
    this.monitor = this.ctx.createGain();
    this.monitor.gain.value = this.monitorEnabled ? 1 : 0;
    this.monitor.connect(this.master);
    const latency = this.ctx.baseLatency + (this.ctx.outputLatency || 0);
    this.latencyFrames = Math.round(latency * this.sessionRate);
  }

  async openInput(system, sampleRate) {
    if (this.opening) await this.opening;
    if (this.stream && this.system === system && this.stream.active) return;
    this.opening = this.openStream(system, sampleRate);
    try {
      await this.opening;
    } finally {
      this.opening = null;
    }
  }

  async openStream(system, sampleRate) {
    if (!navigator.mediaDevices) {
      throw new Error("This browser does not expose an audio input.");
    }
    await this.ensureContext(sampleRate);
    if (this.stream && this.system === system && this.stream.active) return;
    this.releaseInput();
    const audio = { echoCancellation: false, noiseSuppression: false, autoGainControl: false, channelCount: { ideal: 2 } };
    const stream = system
      ? await navigator.mediaDevices.getDisplayMedia({ video: true, audio })
      : await navigator.mediaDevices.getUserMedia({ audio, video: false });
    if (system) {
      for (const track of stream.getVideoTracks()) track.stop();
    }
    this.stream = stream;
    this.system = system;
    this.input = this.ctx.createMediaStreamSource(stream);
    this.splitter = this.ctx.createChannelSplitter(2);
    this.input.connect(this.splitter);
    this.tuners = [0, 1].map((channel) => {
      const analyser = this.ctx.createAnalyser();
      analyser.fftSize = 2048;
      const sink = this.ctx.createGain();
      sink.gain.value = 0;
      this.splitter.connect(analyser, channel);
      analyser.connect(sink);
      sink.connect(this.master);
      return { analyser, sink, buffer: new Float32Array(analyser.fftSize) };
    });
  }

  releaseInput() {
    if (this.stream) {
      for (const track of this.stream.getTracks()) track.stop();
    }
    if (this.input) {
      try { this.input.disconnect(); } catch (error) { /* already gone */ }
    }
    if (this.splitter) {
      try { this.splitter.disconnect(); } catch (error) { /* already gone */ }
    }
    if (this.tuners) {
      for (const tuner of this.tuners) {
        try { tuner.analyser.disconnect(); tuner.sink.disconnect(); } catch (error) { /* already gone */ }
      }
    }
    for (const chain of this.chains.values()) chain.dispose();
    this.chains.clear();
    this.stream = null;
    this.input = null;
    this.splitter = null;
    this.tuners = null;
  }

  setMonitor(enabled) {
    this.monitorEnabled = enabled;
    if (this.monitor) this.monitor.gain.value = enabled ? 1 : 0;
  }

  setMasterVolume(level) {
    this.masterLevel = Math.min(1, Math.max(0, Number(level) || 0));
    if (!this.master || !this.ctx) return;
    const now = this.ctx.currentTime;
    this.master.gain.cancelScheduledValues(now);
    this.master.gain.setTargetAtTime(this.masterLevel, now, 0.02);
  }

  syncTakeLevels(state) {
    if (!this.ctx || !this.takeGains.size) return;
    const audible = new Set(audibleTakes(state).map(({ take }) => take.id));
    const takes = new Map(state.takes.map((take) => [take.id, take]));
    const now = this.ctx.currentTime;
    for (const [takeId, gain] of this.takeGains) {
      const take = takes.get(takeId);
      if (!take) {
        this.takeGains.delete(takeId);
        gain.gain.setTargetAtTime(0, now, 0.015);
        continue;
      }
      const level = audible.has(takeId) ? Math.pow(10, take.gainDb / 20) : 0;
      gain.gain.setTargetAtTime(level, now, 0.015);
    }
  }

  syncMonitoring(state) {
    this.lastState = state;
    if (!this.ctx) return;
    this.syncPlaybackChains(state);
    if (!this.splitter) return;
    const tracks = state.tracks.filter((track) => track.source === "input");
    const wanted = new Set(tracks.map((track) => track.id));
    for (const [id, chain] of this.chains) {
      if (!wanted.has(id)) {
        chain.dispose();
        this.chains.delete(id);
      }
    }
    for (const track of tracks) {
      const signature = JSON.stringify(track.effects);
      let chain = this.chains.get(track.id);
      if (!chain || chain.signature !== signature) {
        if (chain) chain.dispose();
        const built = buildEffectsChain(this.ctx, track.effects);
        const analyser = this.ctx.createAnalyser();
        analyser.fftSize = 2048;
        analyser.smoothingTimeConstant = 0.65;
        analyser.minDecibels = -90;
        analyser.maxDecibels = -10;
        const inputAnalyser = this.ctx.createAnalyser();
        inputAnalyser.fftSize = 2048;
        const gain = this.ctx.createGain();
        const panner = this.ctx.createStereoPanner();
        panner.pan.value = trackPan(track);
        built.output.connect(analyser);
        analyser.connect(gain);
        gain.connect(panner);
        panner.connect(this.monitor);
        this.splitter.connect(built.input, Math.min(track.channel, 1));
        this.splitter.connect(inputAnalyser, Math.min(track.channel, 1));
        chain = {
          ...built,
          analyser,
          inputAnalyser,
          gain,
          panner,
          signature,
          meterBuffer: new Float32Array(analyser.fftSize),
          inputBuffer: new Float32Array(inputAnalyser.fftSize),
          spectrumBuffer: new Float32Array(analyser.frequencyBinCount),
          dispose() {
            built.dispose();
            for (const node of [analyser, inputAnalyser, gain, panner]) {
              try {
                node.disconnect();
              } catch (error) {
                // Nodes may already be detached by the browser.
              }
            }
          },
        };
        this.chains.set(track.id, chain);
      }
      chain.gain.gain.value = track.armed && !track.muted ? trackLevel(track) : 0;
      chain.panner.pan.setTargetAtTime(trackPan(track), this.ctx.currentTime, 0.015);
    }
  }

  // Recorded takes play through the same per-track chain as live monitoring, so
  // an effects or volume change is heard whether the track is live or already
  // recorded.
  syncPlaybackChains(state) {
    const tracks = new Map(state.tracks.map((track) => [track.id, track]));
    const now = this.ctx.currentTime;
    for (const track of tracks.values()) {
      const volume = this.playbackVolumes.get(track.id);
      if (volume) volume.gain.setTargetAtTime(trackLevel(track), now, 0.015);
      if (this.playbackChains.has(track.id)) this.playbackChain(track);
      const chain = this.playbackChains.get(track.id);
      if (chain) chain.panner.pan.setTargetAtTime(trackPan(track), now, 0.015);
    }
    for (const [id, chain] of this.playbackChains) {
      if (tracks.has(id)) continue;
      chain.dispose();
      this.playbackChains.delete(id);
      const volume = this.playbackVolumes.get(id);
      if (volume) volume.disconnect();
      this.playbackVolumes.delete(id);
      const playback = this.playbackAnalysers.get(id);
      if (playback) playback.analyser.disconnect();
      this.playbackAnalysers.delete(id);
    }
  }

  playbackChain(track) {
    const signature = JSON.stringify(track.effects);
    const current = this.playbackChains.get(track.id);
    if (current && current.signature === signature) return current;
    const built = buildEffectsChain(this.ctx, track.effects);
    const panner = this.ctx.createStereoPanner();
    panner.pan.value = trackPan(track);
    built.output.connect(panner);
    panner.connect(this.master);
    const volume = this.trackVolume(track);
    volume.disconnect();
    volume.connect(built.input);
    if (current) current.dispose();
    const chain = {
      ...built,
      signature,
      panner,
      dispose() {
        built.dispose();
        panner.disconnect();
      },
    };
    this.playbackChains.set(track.id, chain);
    return chain;
  }

  trackVolume(track) {
    let node = this.playbackVolumes.get(track.id);
    if (!node) {
      node = this.ctx.createGain();
      this.playbackVolumes.set(track.id, node);
    }
    node.gain.value = trackLevel(track);
    return node;
  }

  playbackAnalyser(track) {
    let entry = this.playbackAnalysers.get(track.id);
    if (!entry) {
      const analyser = this.ctx.createAnalyser();
      analyser.fftSize = 2048;
      entry = { analyser, buffer: new Float32Array(analyser.fftSize) };
      this.playbackAnalysers.set(track.id, entry);
    }
    return entry.analyser;
  }

  meter(trackId) {
    const chain = this.chains.get(trackId);
    if (!chain) return 0;
    chain.analyser.getFloatTimeDomainData(chain.meterBuffer);
    let peak = 0;
    for (let index = 0; index < chain.meterBuffer.length; index++) {
      const value = Math.abs(chain.meterBuffer[index]);
      if (value > peak) peak = value;
    }
    return peak;
  }

  spectrum(trackId) {
    const chain = this.chains.get(trackId);
    if (!chain) return null;
    if (!chain.spectrumBuffer || chain.spectrumBuffer.length !== chain.analyser.frequencyBinCount) {
      chain.spectrumBuffer = new Float32Array(chain.analyser.frequencyBinCount);
    }
    chain.analyser.getFloatFrequencyData(chain.spectrumBuffer);
    return { bins: chain.spectrumBuffer, sampleRate: this.ctx.sampleRate };
  }

  // level reports the dry signal feeding a track's compressor, live while
  // monitoring or from the take during playback, so the compressor graph can
  // map it onto the transfer curve.
  level(trackId) {
    const playback = this.playbackAnalysers.get(trackId);
    const chain = this.chains.get(trackId);
    if (playback && this.playing && !this.capture) {
      return this.readLevel(playback.analyser, playback.buffer);
    }
    if (chain?.inputAnalyser) {
      return this.readLevel(chain.inputAnalyser, chain.inputBuffer);
    }
    if (playback) return this.readLevel(playback.analyser, playback.buffer);
    return null;
  }

  readLevel(analyser, buffer) {
    analyser.getFloatTimeDomainData(buffer);
    let sum = 0;
    for (let index = 0; index < buffer.length; index++) {
      sum += buffer[index] * buffer[index];
    }
    return { rms: Math.sqrt(sum / buffer.length) };
  }

  tuner(channel) {
    if (!this.tuners) return null;
    const tuner = this.tuners[Math.min(Math.max(channel, 0), 1)];
    tuner.analyser.getFloatTimeDomainData(tuner.buffer);
    return noteReading(detectPitch(tuner.buffer, this.ctx.sampleRate));
  }

  buffer(file, audio) {
    const cached = this.buffers.get(file);
    if (cached && cached.audio === audio) return cached.buffer;
    const frames = Math.floor(audio.samples.length / audio.channels);
    const rate = this.ctx.sampleRate;
    const outputFrames = Math.max(1, Math.round((frames * rate) / audio.sampleRate));
    const buffer = this.ctx.createBuffer(audio.channels, outputFrames, rate);
    const step = audio.sampleRate / rate;
    for (let channel = 0; channel < audio.channels; channel++) {
      const target = buffer.getChannelData(channel);
      for (let frame = 0; frame < outputFrames; frame++) {
        const position = Math.min(frames - 1, frame * step);
        const index = Math.floor(position);
        const next = Math.min(frames - 1, index + 1);
        const mix = position - index;
        target[frame] = audio.samples[index * audio.channels + channel] * (1 - mix)
          + audio.samples[next * audio.channels + channel] * mix;
      }
    }
    this.buffers.set(file, { audio, buffer });
    return buffer;
  }

  invalidate(file) {
    this.buffers.delete(file);
  }

  async start(state, pcm, fromFrame, options = {}) {
    await this.ensureContext(state.sampleRate);
    this.stopSources();
    const when = this.ctx.currentTime + 0.12;
    this.anchorTime = when;
    this.anchorFrame = Math.max(0, Math.round(fromFrame));
    this.playing = true;
    this.schedulePlayback(state, pcm, this.anchorFrame, when);
    this.startClick();
    if (options.record) {
      this.startCapture(options.slots || [], when);
    }
    return when;
  }

  schedulePlayback(state, pcm, fromFrame, when) {
    const rate = this.ctx.sampleRate;
    for (const { track, take } of audibleTakes(state)) {
      const audio = pcm.get(take.file);
      if (!audio) continue;
      const buffer = this.buffer(take.file, audio);
      const trimStart = Math.max(0, take.trim.startFrame);
      const trimEnd = take.trim.endFrame > 0 ? take.trim.endFrame : take.lengthFrames;
      const length = trimEnd - trimStart;
      const startAt = take.startFrame + trimStart;
      if (length <= 0 || startAt + length <= fromFrame) continue;
      const skip = Math.max(0, fromFrame - startAt);
      const source = this.ctx.createBufferSource();
      source.buffer = buffer;
      const gain = this.ctx.createGain();
      gain.gain.value = Math.pow(10, take.gainDb / 20);
      this.takeGains.set(take.id, gain);
      this.playbackChain(track);
      const volume = this.trackVolume(track);
      source.connect(gain);
      gain.connect(this.playbackAnalyser(track));
      gain.connect(volume);
      source.start(when + Math.max(0, (startAt - fromFrame) / rate), (trimStart + skip) / rate, (length - skip) / rate);
      this.sources.push(source, gain);
    }
  }

  startCapture(slots, when) {
    if (!slots.length) throw new Error("Arm a track before recording.");
    const system = slots.length === 1 && slots[0].kind === "system";
    const node = new AudioWorkletNode(this.ctx, "gorecord-capture", {
      numberOfInputs: 1,
      numberOfOutputs: 1,
      outputChannelCount: [1],
    });
    const silent = this.ctx.createGain();
    silent.gain.value = 0;
    node.connect(silent);
    silent.connect(this.master);
    let merger = null;
    if (system) {
      this.input.connect(node);
    } else {
      merger = this.ctx.createChannelMerger(slots.length);
      slots.forEach((slot, index) => this.splitter.connect(merger, Math.min(slot.channel, 1), index));
      merger.connect(node);
    }
    this.capture = {
      node,
      silent,
      merger,
      slots,
      chunks: [],
      frames: 0,
      peaks: slots.map(() => []),
      resolveStopped: null,
    };
    node.port.onmessage = (event) => this.onCaptureMessage(event.data);
    node.port.postMessage({ command: "start", at: when });
  }

  onCaptureMessage(message) {
    const capture = this.capture;
    if (!capture) return;
    if (message.type === "audio") {
      message.channels.forEach((channel, index) => {
        if (!capture.chunks[index]) capture.chunks[index] = [];
        capture.chunks[index].push(channel);
      });
      capture.frames += message.channels[0].length;
      message.channels.forEach((samples, index) => {
        let low = 0;
        let high = 0;
        for (let frame = 0; frame < samples.length; frame++) {
          if (samples[frame] < low) low = samples[frame];
          if (samples[frame] > high) high = samples[frame];
        }
        capture.peaks[index].push(low, high);
      });
      return;
    }
    if (message.type === "stopped" && capture.resolveStopped) {
      capture.resolveStopped();
    }
  }

  takeCaptures(dropFrames) {
    const capture = this.capture;
    const kept = Math.max(0, capture.frames - dropFrames);
    return capture.slots.map((slot, slotIndex) => {
      const channels = slot.kind === "system" ? 2 : 1;
      const output = new Float32Array(kept * channels);
      const chunkCount = capture.chunks[0] ? capture.chunks[0].length : 0;
      let position = 0;
      let write = 0;
      for (let chunk = 0; chunk < chunkCount; chunk++) {
        const block = capture.chunks[0][chunk];
        for (let frame = 0; frame < block.length; frame++) {
          if (position < dropFrames) {
            position++;
            continue;
          }
          for (let channel = 0; channel < channels; channel++) {
            const source = capture.chunks[slot.kind === "system" ? channel : slotIndex];
            output[write * channels + channel] = source ? source[chunk][frame] : 0;
          }
          write++;
          position++;
        }
      }
      const samples = resample(output.subarray(0, write * channels), channels, this.ctx.sampleRate, this.sessionRate);
      return { slot, channels, frames: write, sampleRate: this.sessionRate, samples };
    });
  }

  async stopRecording(dropFrames = 0) {
    const capture = this.capture;
    if (!capture) return null;
    const stopped = new Promise((resolve) => { capture.resolveStopped = resolve; });
    capture.node.port.postMessage({ command: "stop" });
    await stopped;
    const result = this.takeCaptures(dropFrames);
    this.capture = null;
    this.stop();
    try {
      capture.node.disconnect();
      capture.silent.disconnect();
      if (capture.merger) capture.merger.disconnect();
    } catch (error) {
      // Nodes may already be detached by the browser.
    }
    return result;
  }

  livePeaks() {
    if (!this.capture) return [];
    return this.capture.slots.map((slot, index) => ({
      trackId: slot.trackId,
      peaks: this.capture.peaks[index] || [],
      blockFrames: BLOCK_FRAMES,
    }));
  }

  position() {
    if (!this.playing || !this.ctx) return this.idleFrame;
    const elapsed = Math.max(0, this.ctx.currentTime - this.anchorTime);
    return this.anchorFrame + elapsed * this.sessionRate;
  }

  stopSources() {
    for (const node of this.sources) {
      try {
        if (typeof node.stop === "function") node.stop();
        node.disconnect();
      } catch (error) {
        // A source that already ended cannot be stopped again.
      }
    }
    this.sources = [];
    this.takeGains.clear();
    this.stopClick();
  }

  stop() {
    const frame = this.position();
    this.stopSources();
    this.playing = false;
    this.idleFrame = frame;
  }

  setClick(click) {
    this.click = click;
    if (this.playing) this.startClick();
  }

  // countIn plays a metronome lead-in before recording starts. It is audible
  // even when the click is switched off, because a count-in must be heard.
  async countIn({ bpm = 120, beatsPerBar = 4, bars = 1, onBeat = null } = {}) {
    await this.ensureContext(this.sessionRate);
    const meter = Math.max(2, beatsPerBar || 4);
    const beats = Math.max(1, bars) * meter;
    const beatSeconds = 60 / Math.min(300, Math.max(20, bpm || 120));
    const start = this.ctx.currentTime + 0.08;
    for (let beat = 0; beat < beats; beat++) {
      const when = start + beat * beatSeconds;
      this.clickAt(when, beat % meter === 0);
      if (onBeat) {
        const delay = Math.max(0, (when - this.ctx.currentTime) * 1000);
        window.setTimeout(() => onBeat(beats - beat), delay);
      }
    }
    const remaining = Math.max(0, (start + beats * beatSeconds - this.ctx.currentTime) * 1000);
    await new Promise((resolve) => window.setTimeout(resolve, remaining + 40));
  }

  startClick() {
    this.stopClick();
    if (!this.click || !this.click.Enabled) return;
    this.scheduleClick();
    this.clickTimer = window.setInterval(() => this.scheduleClick(), 120);
  }

  stopClick() {
    if (this.clickTimer) window.clearInterval(this.clickTimer);
    this.clickTimer = 0;
  }

  scheduleClick() {
    if (!this.playing || !this.ctx) return;
    const rate = this.ctx.sampleRate;
    const beatFrames = (rate * 60) / Math.max(20, this.click.BPM || 120);
    let beat = Math.ceil(this.position() / beatFrames);
    for (let guard = 0; guard < 64; guard++) {
      const time = this.anchorTime + (beat * beatFrames - this.anchorFrame) / rate;
      if (time > this.ctx.currentTime + 0.25) break;
      if (time >= this.ctx.currentTime - 0.01) {
        this.clickAt(time, beat % Math.max(2, this.click.BeatsPerBar || 4) === 0);
      }
      beat++;
    }
  }

  clickAt(when, accent) {
    const oscillator = this.ctx.createOscillator();
    const gain = this.ctx.createGain();
    oscillator.frequency.value = accent ? 1568 : 1046;
    gain.gain.setValueAtTime(0.0001, when);
    gain.gain.exponentialRampToValueAtTime(0.22, when + 0.001);
    gain.gain.exponentialRampToValueAtTime(0.0001, when + 0.04);
    oscillator.connect(gain);
    gain.connect(this.master);
    oscillator.start(when);
    oscillator.stop(when + 0.05);
  }

  dispose() {
    this.stop();
    this.releaseInput();
    if (this.ctx) this.ctx.close().catch(() => {});
    this.ctx = null;
    this.buffers.clear();
    this.chains.clear();
    this.playbackChains.clear();
    this.playbackVolumes.clear();
    this.playbackAnalysers.clear();
    this.sources = [];
  }
}
