// Browser audio engine. Capture and monitoring run through the Web Audio API;
// take audio is handed back to the Go core for persistence and export.

import { buildEffectsChain } from "./effects.js";
import { detectPitch, noteReading } from "./tuner.js";
import { audibleTakes } from "./model.js";
import { estimateClockRatio } from "./clock.js";

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

// Every track plays through its own chain, both when monitored live and when
// its recorded takes are played back.
function trackChain(context, track) {
  return buildEffectsChain(context, track.effects);
}

export class AudioEngine {
  constructor() {
    this.ctx = null;
    this.sessionRate = 48000;
    this.master = null;
    this.monitor = null;
    this.monitorEnabled = true;
    this.masterLevel = 1;
    this.outputIds = [""];
    this.extraOutputs = new Map();
    this.primaryOutput = null;
    this.outputQueue = Promise.resolve();
    this.inputs = new Map();
    this.clockProbes = new Map();
    this.chains = new Map();
    this.playbackChains = new Map();
    this.playbackVolumes = new Map();
    this.playbackAnalysers = new Map();
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
    this.disposed = false;
    this.click = { Enabled: false, BPM: 120, BeatsPerBar: 4 };
    this.clickTimer = 0;
    this.lastState = null;
  }

  async ensureContext(sampleRate) {
    if (this.disposed) throw new Error("The audio engine was closed.");
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
    const applyOutputs = () => this.applyOutputs(this.outputIds);
    this.outputQueue = this.outputQueue.then(applyOutputs, applyOutputs);
    this.outputQueue.catch(() => {});
  }

  // Monitoring and capture may need the microphone and PC audio at the same
  // time. Sources are opened independently and stay alive while either is used.
  async openSources(kinds, sampleRate) {
    const wanted = [...new Set(kinds)].filter((kind) => kind === "input" || kind === "system");
    if (!wanted.length || this.disposed) return {};
    if (this.opening) await this.opening;
    if (this.disposed) return {};
    await this.ensureContext(sampleRate);
    if (this.disposed) return {};
    const missing = wanted.filter((kind) => {
      const source = this.inputs.get(kind);
      return !source || !source.stream.active;
    });
    if (!missing.length) return {};
    this.opening = this.openStreams(missing, sampleRate);
    try {
      return await this.opening;
    } finally {
      this.opening = null;
    }
  }

  // The display picker needs the user gesture, so ask for PC audio first when
  // both sources are requested.
  async openStreams(kinds, sampleRate) {
    const failures = {};
    const ordered = [...kinds].sort((left, right) => (left === "system" ? -1 : right === "system" ? 1 : 0));
    for (const kind of ordered) {
      try {
        await this.openStream(kind, sampleRate);
      } catch (error) {
        failures[kind] = error;
      }
    }
    return failures;
  }

  async openStream(kind, sampleRate) {
    if (this.disposed) throw new Error("The audio engine was closed.");
    if (!navigator.mediaDevices) {
      throw new Error("This browser does not expose an audio input.");
    }
    await this.ensureContext(sampleRate);
    const existing = this.inputs.get(kind);
    if (existing && existing.stream.active) return;
    if (existing) this.releaseSource(kind);
    const audio = { echoCancellation: false, noiseSuppression: false, autoGainControl: false, channelCount: { ideal: 2 } };
    const stream = kind === "system"
      ? await navigator.mediaDevices.getDisplayMedia({ video: true, audio })
      : await navigator.mediaDevices.getUserMedia({ audio, video: false });
    if (this.disposed || !this.ctx) {
      for (const track of stream.getTracks()) track.stop();
      throw new Error("The audio engine was closed.");
    }
    if (kind === "system") {
      for (const track of stream.getVideoTracks()) track.stop();
    }
    if (!stream.getAudioTracks().length) {
      for (const track of stream.getTracks()) track.stop();
      throw new Error(kind === "system"
        ? "No tab audio was shared. Choose a tab and enable its audio."
        : "No audio input was found on this device.");
    }
    const node = this.ctx.createMediaStreamSource(stream);
    let splitter = null;
    let tuners = null;
    if (kind === "input") {
      splitter = this.ctx.createChannelSplitter(2);
      node.connect(splitter);
      tuners = [0, 1].map((channel) => {
        const analyser = this.ctx.createAnalyser();
        analyser.fftSize = 2048;
        const sink = this.ctx.createGain();
        sink.gain.value = 0;
        splitter.connect(analyser, channel);
        analyser.connect(sink);
        sink.connect(this.master);
        return { analyser, sink, buffer: new Float32Array(analyser.fftSize) };
      });
    }
    this.inputs.set(kind, { stream, node, splitter, tuners });
    this.clearLiveChains();
  }

  releaseSource(kind) {
    this.stopClockProbe(kind);
    const source = this.inputs.get(kind);
    if (!source) return;
    this.inputs.delete(kind);
    for (const track of source.stream.getTracks()) track.stop();
    try { source.node.disconnect(); } catch (error) { /* already gone */ }
    if (source.splitter) {
      try { source.splitter.disconnect(); } catch (error) { /* already gone */ }
    }
    if (source.tuners) {
      for (const tuner of source.tuners) {
        try { tuner.analyser.disconnect(); tuner.sink.disconnect(); } catch (error) { /* already gone */ }
      }
    }
    this.clearLiveChains();
  }

  releaseInput() {
    for (const kind of [...this.inputs.keys()]) this.releaseSource(kind);
  }

  clearLiveChains() {
    for (const chain of this.chains.values()) chain.dispose();
    this.chains.clear();
  }

  startClockProbe(kind, source) {
    if (typeof Worker !== "function") return null;
    this.stopClockProbe(kind);
    const track = source?.stream?.getAudioTracks?.()[0]?.clone?.();
    if (!track) return null;
    let worker;
    try {
      worker = new Worker("clock-probe-worker.js");
    } catch (error) {
      track.stop();
      return null;
    }
    let nominalRate = this.ctx.sampleRate;
    try {
      const settings = track.getSettings ? track.getSettings() : null;
      nominalRate = Number(settings?.sampleRate) || nominalRate;
    } catch (error) {
      // Settings are optional; the context rate is a safe fallback.
    }
    const probe = { worker, track, nominalRate, samples: [{ frames: 0, at: performance.now() }] };
    worker.onmessage = (event) => {
      const message = event.data || {};
      if (message.type !== "progress") return;
      if (Number(message.sampleRate) > 0) probe.nominalRate = Number(message.sampleRate);
      probe.samples.push({ frames: Number(message.frames) || 0, at: performance.now() });
    };
    worker.onerror = () => {};
    try {
      worker.postMessage({ command: "start", track }, [track]);
    } catch (error) {
      try { worker.terminate(); } catch (terminateError) { /* already closed */ }
      try { track.stop(); } catch (stopError) { /* transfer already detached it */ }
      return null;
    }
    this.clockProbes.set(kind, probe);
    return probe;
  }

  stopClockProbe(kind) {
    const probe = this.clockProbes.get(kind);
    if (!probe) return;
    this.clockProbes.delete(kind);
    try { probe.worker.postMessage({ command: "stop" }); } catch (error) { /* worker already closed */ }
    // Give the worker a moment to stop its cloned track before tearing it down.
    const terminate = () => {
      try { probe.worker.terminate(); } catch (error) { /* worker already closed */ }
    };
    if (typeof setTimeout === "function") setTimeout(terminate, 100);
    else terminate();
    try { probe.track.stop(); } catch (error) { /* transferred track already stopped */ }
  }

  stopClockProbes() {
    for (const kind of [...this.clockProbes.keys()]) this.stopClockProbe(kind);
  }

  clockRatio(kind) {
    const probe = this.clockProbes.get(kind);
    return probe ? estimateClockRatio(probe.samples, probe.nominalRate) : 1;
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

  // A context has one hardware destination. The first selected output uses it
  // directly for low latency; extra outputs receive the same master mix through
  // a MediaStream and an audio element pinned to that device.
  setOutputs(ids) {
    const selected = [];
    for (const id of ids || []) {
      if (typeof id === "string" && !selected.includes(id)) selected.push(id);
    }
    this.outputIds = selected.length ? selected : [""];
    const apply = () => this.applyOutputs(this.outputIds);
    this.outputQueue = this.outputQueue.then(apply, apply);
    return this.outputQueue;
  }

  async applyOutputs(ids) {
    if (!this.ctx || !this.master) return;
    const primary = ids[0] || "";
    const extras = ids.slice(1);
    let failure = null;
    try {
      await this.routePrimary(primary);
    } catch (error) {
      failure = error;
    }
    for (const id of [...this.extraOutputs.keys()]) {
      if (!extras.includes(id)) this.closeExtraOutput(id);
    }
    for (const id of extras) {
      try {
        await this.openExtraOutput(id);
      } catch (error) {
        failure = failure || error;
      }
    }
    if (failure) throw failure;
  }

  async routePrimary(id) {
    if (this.primaryOutput === id) return;
    if (!id && this.primaryOutput === null) {
      this.primaryOutput = "";
      return;
    }
    if (typeof this.ctx.setSinkId !== "function") {
      if (id) throw new Error("This browser cannot choose a master output device.");
      this.primaryOutput = "";
      return;
    }
    await this.ctx.setSinkId(id);
    this.primaryOutput = id;
  }

  async openExtraOutput(id) {
    let output = this.extraOutputs.get(id);
    if (!output) {
      if (typeof this.ctx.createMediaStreamDestination !== "function") {
        throw new Error("This browser cannot send the master to several outputs.");
      }
      const destination = this.ctx.createMediaStreamDestination();
      this.master.connect(destination);
      const element = document.createElement("audio");
      element.autoplay = true;
      element.playsInline = true;
      element.hidden = true;
      element.srcObject = destination.stream;
      document.body.append(element);
      output = { destination, element, sinkId: null };
      this.extraOutputs.set(id, output);
    }
    if (typeof output.element.setSinkId !== "function") {
      this.closeExtraOutput(id);
      throw new Error("This browser cannot send the master to more than one output.");
    }
    if (output.sinkId !== id) {
      await output.element.setSinkId(id);
      output.sinkId = id;
    }
    await output.element.play();
  }

  closeExtraOutput(id) {
    const output = this.extraOutputs.get(id);
    if (!output) return;
    this.extraOutputs.delete(id);
    try { output.element.pause(); } catch (error) { /* already gone */ }
    output.element.srcObject = null;
    output.element.remove();
    try { this.master.disconnect(output.destination); } catch (error) { /* already gone */ }
    try { output.destination.disconnect(); } catch (error) { /* already gone */ }
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

  syncMonitoring(state, monitored = null) {
    this.lastState = state;
    if (!this.ctx) return;
    this.syncPlaybackChains(state);
    const tracks = state.tracks.filter((track) => {
      if (track.source !== "input" && track.source !== "system") return false;
      return !monitored || monitored.has(track.id);
    });
    const wanted = new Set(tracks.map((track) => track.id));
    for (const [id, chain] of this.chains) {
      if (!wanted.has(id)) {
        chain.dispose();
        this.chains.delete(id);
      }
    }
    for (const track of tracks) {
      const signature = JSON.stringify({ effects: track.effects, source: track.source, channel: track.channel });
      let chain = this.chains.get(track.id);
      if (!chain || chain.signature !== signature) {
        if (chain) chain.dispose();
        const built = trackChain(this.ctx, track);
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
        const source = this.inputs.get(track.source);
        if (track.source === "system" && source?.node) {
          source.node.connect(built.input);
          source.node.connect(inputAnalyser);
        } else if (source?.splitter) {
          const channel = Math.min(track.channel, 1);
          source.splitter.connect(built.input, channel);
          source.splitter.connect(inputAnalyser, channel);
        }
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
      const audible = !track.muted && (!state.anySolo || track.soloed);
      chain.gain.gain.value = audible ? trackLevel(track) : 0;
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
    const signature = JSON.stringify({ effects: track.effects, source: track.source });
    const current = this.playbackChains.get(track.id);
    if (current && current.signature === signature) return current;
    const built = trackChain(this.ctx, track);
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
    const source = this.inputs.get("input");
    if (!source?.tuners) return null;
    const tuner = source.tuners[Math.min(Math.max(channel, 0), 1)];
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
      const startAt = take.startFrame;
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
    const groups = new Map();
    this.capture = { groups, slots, pendingStops: 0, resolveStopped: null };
    for (const slot of slots) {
      if (groups.has(slot.kind)) continue;
      const groupSlots = slots.filter((item) => item.kind === slot.kind);
      const group = this.openCaptureGroup(slot.kind, groupSlots);
      groups.set(slot.kind, group);
      this.capture.pendingStops += 1;
    }
    for (const group of groups.values()) {
      group.probe = this.startClockProbe(group.kind, this.inputs.get(group.kind));
      group.node.port.postMessage({ command: "start", at: when });
    }
  }

  openCaptureGroup(kind, slots) {
    const source = this.inputs.get(kind);
    if (!source) throw new Error("The audio source is not open.");
    if (kind !== "system" && !source.splitter) throw new Error("The audio input has no channels.");
    const node = new AudioWorkletNode(this.ctx, "fanamix-capture", {
      numberOfInputs: 1,
      numberOfOutputs: 1,
      outputChannelCount: [1],
    });
    const silent = this.ctx.createGain();
    silent.gain.value = 0;
    node.connect(silent);
    silent.connect(this.master);
    let merger = null;
    if (kind === "system") {
      source.node.connect(node);
    } else {
      merger = this.ctx.createChannelMerger(slots.length);
      slots.forEach((slot, index) => source.splitter.connect(merger, Math.min(slot.channel, 1), index));
      merger.connect(node);
    }
    const group = {
      kind,
      node,
      silent,
      merger,
      slots,
      chunks: [],
      frames: 0,
      peaks: slots.map(() => []),
      stopped: false,
      probe: null,
    };
    node.port.onmessage = (event) => this.onCaptureMessage(group, event.data);
    return group;
  }

  onCaptureMessage(group, message) {
    const capture = this.capture;
    if (!capture || capture.groups.get(group.kind) !== group) return;
    if (message.type === "audio" && message.channels?.length) {
      message.channels.forEach((channel, index) => {
        if (!group.chunks[index]) group.chunks[index] = [];
        group.chunks[index].push(channel);
      });
      group.frames += message.channels[0].length;
      message.channels.forEach((samples, index) => {
        let low = 0;
        let high = 0;
        for (let frame = 0; frame < samples.length; frame++) {
          if (samples[frame] < low) low = samples[frame];
          if (samples[frame] > high) high = samples[frame];
        }
        group.peaks[index]?.push(low, high);
      });
      return;
    }
    if (message.type !== "stopped" || group.stopped) return;
    group.stopped = true;
    capture.pendingStops -= 1;
    if (capture.pendingStops <= 0 && capture.resolveStopped) {
      capture.resolveStopped();
    }
  }

  takeCaptures(dropFrames) {
    const capture = this.capture;
    return capture.slots.map((slot) => {
      const group = capture.groups.get(slot.kind);
      if (!group) throw new Error("The capture group disappeared.");
      return this.takeFromGroup(group, slot, dropFrames);
    });
  }

  takeFromGroup(group, slot, dropFrames) {
    const channels = slot.kind === "system" ? 2 : 1;
    const localIndex = group.slots.indexOf(slot);
    const kept = Math.max(0, group.frames - dropFrames);
    const output = new Float32Array(kept * channels);
    const chunkCount = group.chunks[0] ? group.chunks[0].length : 0;
    let position = 0;
    let write = 0;
    for (let chunk = 0; chunk < chunkCount; chunk++) {
      const block = group.chunks[0][chunk];
      for (let frame = 0; frame < block.length; frame++) {
        if (position < dropFrames) {
          position++;
          continue;
        }
        for (let channel = 0; channel < channels; channel++) {
          const source = group.chunks[slot.kind === "system" ? channel : localIndex];
          output[write * channels + channel] = source ? source[chunk][frame] : 0;
        }
        write++;
        position++;
      }
    }
    const ratio = this.clockRatio(slot.kind);
    // A source clock running fast produces more frames per real second; stretch
    // its take by the same ratio before declaring it at the session rate.
    const targetRate = this.sessionRate * ratio;
    const samples = resample(output.subarray(0, write * channels), channels, this.ctx.sampleRate, targetRate);
    return { slot, channels, frames: samples.length / channels, sampleRate: this.sessionRate, samples };
  }

  async stopRecording(dropFrames = 0) {
    const capture = this.capture;
    if (!capture) return null;
    const stopped = new Promise((resolve) => { capture.resolveStopped = resolve; });
    for (const group of capture.groups.values()) {
      group.node.port.postMessage({ command: "stop" });
    }
    let timeoutId = 0;
    const timeout = new Promise((resolve) => { timeoutId = setTimeout(resolve, 2000); });
    await Promise.race([stopped, timeout]);
    clearTimeout(timeoutId);
    const result = this.takeCaptures(dropFrames);
    this.capture = null;
    this.stopClockProbes();
    this.stop();
    for (const group of capture.groups.values()) {
      try {
        group.node.disconnect();
        group.silent.disconnect();
        if (group.merger) group.merger.disconnect();
      } catch (error) {
        // Nodes may already be detached by the browser.
      }
    }
    return result;
  }

  livePeaks() {
    if (!this.capture) return [];
    return this.capture.slots.map((slot) => {
      const group = this.capture.groups.get(slot.kind);
      if (!group) return { trackId: slot.trackId, peaks: [], blockFrames: BLOCK_FRAMES };
      const index = group.slots.indexOf(slot);
      return { trackId: slot.trackId, peaks: group.peaks[index] || [], blockFrames: BLOCK_FRAMES };
    });
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
    this.disposed = true;
    this.stop();
    this.stopClockProbes();
    this.releaseInput();
    for (const id of [...this.extraOutputs.keys()]) this.closeExtraOutput(id);
    if (this.ctx) this.ctx.close().catch(() => {});
    this.ctx = null;
    this.buffers.clear();
    this.chains.clear();
    this.playbackChains.clear();
    this.playbackVolumes.clear();
    this.playbackAnalysers.clear();
    this.sources = [];
    this.primaryOutput = null;
  }
}
