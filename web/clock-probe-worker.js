// Counts frames delivered by one audio track. The page compares this rate with
// the track's nominal sample rate to estimate clock drift. It is a probe only:
// the audio samples are closed immediately and never recorded here.

let probe = null;

self.onmessage = (event) => {
  const message = event.data || {};
  if (message.command === "stop") {
    stopProbe();
    return;
  }
  if (message.command === "start" && !probe) startProbe(message.track);
};

function startProbe(track) {
  if (typeof MediaStreamTrackProcessor !== "function") {
    stopTrack(track);
    self.postMessage({ type: "unsupported" });
    self.close();
    return;
  }
  let processor;
  try {
    processor = new MediaStreamTrackProcessor({ track });
  } catch (error) {
    stopTrack(track);
    self.postMessage({ type: "unsupported" });
    self.close();
    return;
  }
  probe = {
    track,
    reader: processor.readable.getReader(),
    frames: 0,
    sampleRate: 0,
    stopped: false,
    timer: 0,
  };
  probe.timer = self.setInterval(reportProgress, 250);
  readFrames();
}

function readFrames() {
  const current = probe;
  read(current)
    .then(() => finish(current))
    .catch(() => finish(current));
}

async function read(current) {
  while (!current.stopped) {
    const { done, value } = await current.reader.read();
    if (done) break;
    if (probe !== current) {
      value.close();
      break;
    }
    current.frames += value.numberOfFrames || 0;
    if (!current.sampleRate) current.sampleRate = value.sampleRate || 0;
    value.close();
  }
}

function finish(current) {
  if (probe !== current || current.stopped) return;
  self.clearInterval(current.timer);
  current.stopped = true;
  reportProgress();
  self.postMessage({ type: "stopped", frames: current.frames });
  self.close();
}

function reportProgress() {
  if (!probe) return;
  self.postMessage({ type: "progress", frames: probe.frames, sampleRate: probe.sampleRate });
}

function stopProbe() {
  if (!probe || probe.stopped) return;
  probe.stopped = true;
  self.clearInterval(probe.timer);
  try { probe.reader.cancel(); } catch (error) { /* reader already closed */ }
  stopTrack(probe.track);
  reportProgress();
  self.postMessage({ type: "stopped", frames: probe.frames });
  self.close();
}

function stopTrack(track) {
  try { track?.stop(); } catch (error) { /* transferred track already stopped */ }
}
