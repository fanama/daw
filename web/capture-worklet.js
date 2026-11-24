// Streams captured input to the page in render-quantum blocks. Recording
// starts exactly at the transport anchor so overdubs line up with playback.

class CaptureProcessor extends AudioWorkletProcessor {
  constructor() {
    super();
    this.capturing = false;
    this.startAt = 0;
    this.port.onmessage = (event) => {
      const message = event.data;
      if (message.command === "start") {
        this.startAt = message.at || 0;
        this.capturing = true;
      }
      if (message.command === "stop" && this.capturing) {
        this.capturing = false;
        this.port.postMessage({ type: "stopped" });
      }
    };
  }

  process(inputs) {
    const input = inputs[0];
    if (!this.capturing || !input || input.length === 0) {
      return true;
    }
    const frames = input[0].length;
    let offset = 0;
    if (currentTime < this.startAt) {
      offset = Math.min(frames, Math.round((this.startAt - currentTime) * sampleRate));
      if (offset >= frames) {
        return true;
      }
    }
    const channels = input.map((channel) => channel.slice(offset));
    this.port.postMessage({ type: "audio", channels }, channels.map((channel) => channel.buffer));
    return true;
  }
}

registerProcessor("fanamix-capture", CaptureProcessor);
