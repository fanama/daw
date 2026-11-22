// SVG response graphs for the monitor EQ and compressor. The EQ curve mirrors
// the Web Audio peaking biquads; the compressor graph shows the level transfer
// curve, since compression is gain-based rather than frequency-based.

const SVG_NS = "http://www.w3.org/2000/svg";
const WIDTH = 300;
const HEIGHT = 132;
const LEFT = 32;
const RIGHT = 292;
const TOP = 8;
const BOTTOM = 108;
const PLOT_WIDTH = RIGHT - LEFT;
const PLOT_HEIGHT = BOTTOM - TOP;
const SPECTRUM_POINTS = 180;
const SPECTRUM_FLOOR = -72;
const SPECTRUM_CEIL = 0;
const VOICE_POINTS = 64;
const VOICE_FLOOR = -60;
const VOICE_CEIL = 0;

let graphSequence = 0;

function svgNode(tag, attrs = {}, text) {
  const node = document.createElementNS(SVG_NS, tag);
  for (const [name, value] of Object.entries(attrs)) node.setAttribute(name, value);
  if (text !== undefined) node.textContent = text;
  return node;
}

function addNode(parent, tag, attrs = {}, text) {
  const node = svgNode(tag, attrs, text);
  parent.append(node);
  return node;
}

function axisText(parent, x, y, text, anchor = "middle") {
  return addNode(parent, "text", { x, y, class: "axis-label", "text-anchor": anchor }, text);
}

function frame(kind, label) {
  const id = `fx-graph-${++graphSequence}`;
  const svg = svgNode("svg", { viewBox: `0 0 ${WIDTH} ${HEIGHT}`, role: "img", "aria-label": label });
  const defs = addNode(svg, "defs");
  const clip = addNode(defs, "clipPath", { id: `${id}-clip` });
  addNode(clip, "rect", { x: LEFT, y: TOP, width: PLOT_WIDTH, height: PLOT_HEIGHT });

  const color = kind === "eq" ? "#59d3a8" : "#f0b24a";
  const gradient = addNode(defs, "linearGradient", {
    id: `${id}-fill`, x1: "0", y1: "0", x2: "0", y2: "1",
  });
  addNode(gradient, "stop", { offset: "0", "stop-color": color, "stop-opacity": "0.24" });
  addNode(gradient, "stop", { offset: "1", "stop-color": color, "stop-opacity": "0.02" });

  let spectrumFill = "";
  if (kind === "eq") {
    const spectrumGradient = addNode(defs, "linearGradient", {
      id: `${id}-spectrum`, x1: "0", y1: "0", x2: "0", y2: "1",
    });
    addNode(spectrumGradient, "stop", {
      offset: "0", "stop-color": "#7aa7ff", "stop-opacity": "0.30",
    });
    addNode(spectrumGradient, "stop", {
      offset: "1", "stop-color": "#7aa7ff", "stop-opacity": "0.02",
    });
    spectrumFill = `url(#${id}-spectrum)`;
  }

  const plot = addNode(svg, "g", { "clip-path": `url(#${id}-clip)` });
  return { svg, plot, fill: `url(#${id}-fill)`, spectrumFill };
}

function graphShell(kind, label) {
  const { svg, plot, fill, spectrumFill } = frame(kind, label);
  const element = document.createElement("div");
  element.className = `fx-graph ${kind}`;
  element.append(svg);
  return { svg, plot, fill, spectrumFill, element };
}

function formatHz(frequency) {
  const value = Math.max(20, Math.round(Number(frequency) || 20));
  if (value < 1000) return String(value);
  const thousands = value / 1000;
  return `${Number.isInteger(thousands) ? thousands : thousands.toFixed(1)}k`;
}

function clamp(value, min, max, fallback) {
  const number = Number(value);
  return Math.min(max, Math.max(min, Number.isFinite(number) ? number : fallback));
}

function eqX(frequency) {
  const value = Math.min(20000, Math.max(20, Number(frequency) || 20));
  return LEFT + ((Math.log10(value) - Math.log10(20)) / 3) * PLOT_WIDTH;
}

function eqY(decibels) {
  const value = Math.min(24, Math.max(-24, Number(decibels) || 0));
  return TOP + ((24 - value) / 48) * PLOT_HEIGHT;
}

// peakingDb evaluates the same peaking biquad used by the Web Audio chain.
function peakingDb(frequency, band, sampleRate) {
  const gain = Number(band?.GainDB) || 0;
  if (gain === 0) return 0;
  const rate = Number(sampleRate) || 48000;
  const center = Math.min(rate * 0.49, Math.max(20, Number(band.FrequencyHz) || 1000));
  const q = Math.max(0.1, Number(band.Q) || 0.7);
  const amplitude = Math.pow(10, gain / 40);
  const omega0 = (2 * Math.PI * center) / rate;
  const alpha = Math.sin(omega0) / (2 * q);
  const b0 = 1 + alpha * amplitude;
  const b1 = -2 * Math.cos(omega0);
  const b2 = 1 - alpha * amplitude;
  const a0 = 1 + alpha / amplitude;
  const a1 = b1;
  const a2 = 1 - alpha / amplitude;

  const omega = (2 * Math.PI * frequency) / rate;
  const c1 = Math.cos(omega);
  const s1 = Math.sin(omega);
  const c2 = Math.cos(2 * omega);
  const s2 = Math.sin(2 * omega);
  const numerator = Math.hypot(b0 + b1 * c1 + b2 * c2, -(b1 * s1 + b2 * s2));
  const denominator = Math.max(1e-12, Math.hypot(a0 + a1 * c1 + a2 * c2, -(a1 * s1 + a2 * s2)));
  return 20 * Math.log10(Math.max(1e-9, numerator / denominator));
}

function eqGraph(settings, sampleRate) {
  const { svg, plot, fill, spectrumFill, element } = graphShell(
    "eq", "Equalizer frequency response with live spectrum");
  const rate = Number(sampleRate) || 48000;
  const bandSlots = Array.isArray(settings?.Bands) && settings.Bands.length
    ? settings.Bands.length
    : 5;

  for (const frequency of [20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 20000]) {
    addNode(svg, "line", {
      x1: eqX(frequency), y1: TOP, x2: eqX(frequency), y2: BOTTOM,
      class: frequency === 1000 ? "grid major" : "grid",
    });
  }
  for (const [frequency, text, anchor] of [
    [20, "20", "start"], [100, "100", "middle"], [1000, "1k", "middle"],
    [10000, "10k", "middle"], [20000, "20k", "end"],
  ]) {
    axisText(svg, eqX(frequency), BOTTOM + 12, text, anchor);
  }

  for (const decibels of [24, 12, 0, -12, -24]) {
    const y = eqY(decibels);
    addNode(svg, "line", {
      x1: LEFT, y1: y, x2: RIGHT, y2: y,
      class: decibels === 0 ? "grid major" : "grid",
    });
    axisText(svg, LEFT - 4, y + 3, `${decibels > 0 ? "+" : ""}${decibels}`, "end");
  }

  const spectrumArea = addNode(plot, "path", { class: "spectrum", fill: spectrumFill });
  const spectrumLine = addNode(plot, "path", { class: "spectrum-line" });
  const area = addNode(plot, "path", { class: "area", fill });
  const curve = addNode(plot, "path", { class: "curve" });
  const overlay = addNode(svg, "g");
  const dots = [];
  const labels = [];
  const levels = new Float32Array(SPECTRUM_POINTS).fill(SPECTRUM_FLOOR);
  let hasSpectrum = false;
  for (let index = 0; index < bandSlots; index++) {
    dots.push(addNode(overlay, "circle", { r: "4", class: `band-dot band-${index}` }));
    labels.push(addNode(overlay, "text", {
      class: `band-label band-${index}`, "text-anchor": "middle",
    }));
  }

  const graph = {
    element,
    update(next) {
      const current = next && typeof next === "object" ? next : {};
      element.classList.toggle("off", current.Enabled === false);
      const bands = Array.isArray(current.Bands) ? current.Bands : [];
      const points = [];
      for (let index = 0; index <= 180; index++) {
        const frequency = 20 * Math.pow(1000, index / 180);
        let decibels = 0;
        for (const band of bands) decibels += peakingDb(frequency, band, rate);
        points.push({ x: eqX(frequency), y: eqY(decibels) });
      }

      let path = "";
      points.forEach((point, index) => {
        path += `${index ? "L" : "M"}${point.x.toFixed(1)},${point.y.toFixed(1)}`;
      });
      curve.setAttribute("d", path);
      const zero = eqY(0).toFixed(1);
      area.setAttribute("d",
        `M${LEFT},${zero}` +
        points.map((point) => `L${point.x.toFixed(1)},${point.y.toFixed(1)}`).join("") +
        `L${RIGHT},${zero}Z`);

      for (let index = 0; index < bandSlots; index++) {
        const band = bands[index];
        if (!band) {
          dots[index].setAttribute("display", "none");
          labels[index].setAttribute("display", "none");
          continue;
        }
        const x = eqX(band.FrequencyHz);
        const y = eqY(band.GainDB);
        dots[index].removeAttribute("display");
        labels[index].removeAttribute("display");
        dots[index].setAttribute("cx", x.toFixed(1));
        dots[index].setAttribute("cy", y.toFixed(1));
        labels[index].setAttribute("x", x.toFixed(1));
        labels[index].setAttribute("y", (y < TOP + 14 ? y + 14 : y - 8).toFixed(1));
        labels[index].textContent = formatHz(band.FrequencyHz);
      }
    },
    spectrum(data) {
      const bins = data?.bins;
      if (!bins || !bins.length) {
        for (let index = 0; index < SPECTRUM_POINTS; index++) {
          levels[index] += (SPECTRUM_FLOOR - levels[index]) * 0.18;
        }
        if (!hasSpectrum) return;
        if (levels.every((level) => level <= SPECTRUM_FLOOR + 0.5)) {
          spectrumArea.setAttribute("d", "");
          spectrumLine.setAttribute("d", "");
          hasSpectrum = false;
          return;
        }
      } else {
        hasSpectrum = true;
        const spectrumRate = Math.max(1, Number(data.sampleRate) || rate);
        const nyquist = spectrumRate / 2;
        for (let index = 0; index < SPECTRUM_POINTS; index++) {
          const low = index === 0
            ? 20
            : 20 * Math.pow(1000, (index - 0.5) / (SPECTRUM_POINTS - 1));
          const high = index === SPECTRUM_POINTS - 1
            ? 20000
            : 20 * Math.pow(1000, (index + 0.5) / (SPECTRUM_POINTS - 1));
          const first = Math.min(bins.length - 1,
            Math.max(0, Math.floor((low / nyquist) * bins.length)));
          const last = Math.min(bins.length,
            Math.max(first + 1, Math.ceil((high / nyquist) * bins.length)));
          let decibels = SPECTRUM_FLOOR;
          for (let bin = first; bin < last; bin++) {
            const value = Number(bins[bin]);
            if (Number.isFinite(value)) decibels = Math.max(decibels, value);
          }
          const target = Math.min(SPECTRUM_CEIL, Math.max(SPECTRUM_FLOOR, decibels));
          levels[index] += (target - levels[index]) * (target > levels[index] ? 0.55 : 0.16);
        }
      }

      let line = "";
      let area = `M${LEFT},${BOTTOM}`;
      for (let index = 0; index < SPECTRUM_POINTS; index++) {
        const x = LEFT + (index / (SPECTRUM_POINTS - 1)) * PLOT_WIDTH;
        const y = TOP + ((SPECTRUM_CEIL - levels[index]) / (SPECTRUM_CEIL - SPECTRUM_FLOOR)) * PLOT_HEIGHT;
        line += `${index ? "L" : "M"}${x.toFixed(1)},${y.toFixed(1)}`;
        area += `L${x.toFixed(1)},${y.toFixed(1)}`;
      }
      area += `L${RIGHT},${BOTTOM}Z`;
      spectrumArea.setAttribute("d", area);
      spectrumLine.setAttribute("d", line);
    },
  };
  graph.update(settings);
  return graph;
}

function compressedDb(input, threshold, ratio, knee) {
  const delta = input - threshold;
  if (knee > 0 && Math.abs(delta) <= knee / 2) {
    return input + ((1 / ratio - 1) * Math.pow(delta + knee / 2, 2)) / (2 * knee);
  }
  return delta > 0 ? threshold + delta / ratio : input;
}

function compressorGraph(settings) {
  const { svg, plot, fill, element } = graphShell(
    "comp", "Compressor level transfer curve with a live voice trace");
  const xFor = (decibels) => LEFT + ((Math.min(0, Math.max(-60, decibels)) + 60) / 60) * PLOT_WIDTH;
  const yFor = (decibels) => TOP + ((24 - Math.min(24, Math.max(-60, decibels))) / 84) * PLOT_HEIGHT;
  const curveSettings = { threshold: -18, ratio: 3, knee: 6, makeup: 0 };
  const voiceY = (input) => yFor(compressedDb(
    input, curveSettings.threshold, curveSettings.ratio, curveSettings.knee) + curveSettings.makeup);

  for (const decibels of [-60, -48, -36, -24, -12, 0]) {
    const x = xFor(decibels);
    addNode(svg, "line", { x1: x, y1: TOP, x2: x, y2: BOTTOM, class: "grid" });
    if ([-60, -36, -12, 0].includes(decibels)) {
      const anchor = decibels === -60 ? "start" : decibels === 0 ? "end" : "middle";
      axisText(svg, x, BOTTOM + 12, String(decibels), anchor);
    }
  }
  for (const decibels of [-60, -48, -36, -24, -12, 0, 12, 24]) {
    const y = yFor(decibels);
    addNode(svg, "line", {
      x1: LEFT, y1: y, x2: RIGHT, y2: y,
      class: decibels === 0 ? "grid major" : "grid",
    });
    if ([-60, -24, 0, 24].includes(decibels)) {
      axisText(svg, LEFT - 4, y + 3, `${decibels > 0 ? "+" : ""}${decibels}`, "end");
    }
  }

  addNode(svg, "line", {
    x1: LEFT, y1: yFor(-60), x2: RIGHT, y2: yFor(0), class: "unity",
  });
  const thresholdLine = addNode(svg, "line", { class: "threshold" });
  const thresholdText = addNode(svg, "text", { class: "readout" });
  const area = addNode(plot, "path", { class: "area", fill });
  const curve = addNode(plot, "path", { class: "curve" });
  const thresholdDot = addNode(svg, "circle", { r: "3.5", class: "comp-dot" });

  const guide = addNode(svg, "g", { class: "voice-guide", display: "none" });
  const guideIn = addNode(guide, "line");
  const guideOut = addNode(guide, "line");
  const voiceLayer = addNode(svg, "g", { class: "voice-trace", display: "none" });
  const voiceDots = [];
  for (let index = 0; index < VOICE_POINTS; index++) {
    voiceDots.push(addNode(voiceLayer, "circle", { class: "voice-dot", r: "1.6", display: "none" }));
  }
  const voiceReadout = addNode(svg, "text", {
    class: "readout voice-readout", "text-anchor": "end",
  });
  const voiceLevels = new Float32Array(VOICE_POINTS).fill(VOICE_FLOOR);
  let voiceWrite = 0;
  let voiceFilled = 0;
  let voiceSmooth = VOICE_FLOOR;

  const graph = {
    element,
    update(next) {
      const current = next && typeof next === "object" ? next : {};
      element.classList.toggle("off", current.Enabled === false);
      const threshold = clamp(current.ThresholdDB, -60, 0, -18);
      const ratio = clamp(current.Ratio, 1, 20, 3);
      const knee = clamp(current.KneeDB, 0, 12, 6);
      const makeup = clamp(current.MakeupDB, 0, 24, 0);
      curveSettings.threshold = threshold;
      curveSettings.ratio = ratio;
      curveSettings.knee = knee;
      curveSettings.makeup = makeup;
      const points = [];
      for (let index = 0; index <= 180; index++) {
        const input = -60 + (60 * index) / 180;
        const output = compressedDb(input, threshold, ratio, knee) + makeup;
        points.push({ x: xFor(input), y: yFor(output) });
      }

      let path = "";
      points.forEach((point, index) => {
        path += `${index ? "L" : "M"}${point.x.toFixed(1)},${point.y.toFixed(1)}`;
      });
      curve.setAttribute("d", path);
      area.setAttribute("d",
        `M${LEFT},${BOTTOM}` +
        points.map((point) => `L${point.x.toFixed(1)},${point.y.toFixed(1)}`).join("") +
        `L${RIGHT},${BOTTOM}Z`);

      const x = xFor(threshold);
      thresholdLine.setAttribute("x1", x.toFixed(1));
      thresholdLine.setAttribute("x2", x.toFixed(1));
      thresholdLine.setAttribute("y1", TOP);
      thresholdLine.setAttribute("y2", BOTTOM);
      const labelAtRight = x > RIGHT - 56;
      thresholdText.setAttribute("x", (labelAtRight ? x - 4 : x + 4).toFixed(1));
      thresholdText.setAttribute("y", TOP + 10);
      thresholdText.setAttribute("text-anchor", labelAtRight ? "end" : "start");
      thresholdText.textContent = `T ${Math.round(threshold)}`;

      thresholdDot.setAttribute("cx", x.toFixed(1));
      thresholdDot.setAttribute("cy",
        yFor(compressedDb(threshold, threshold, ratio, knee) + makeup).toFixed(1));
    },
    voice(data) {
      const rms = Number(data?.rms) || 0;
      if (rms < 0.000001) {
        voiceFilled = 0;
        voiceWrite = 0;
        voiceSmooth = VOICE_FLOOR;
        voiceLayer.setAttribute("display", "none");
        guide.setAttribute("display", "none");
        voiceReadout.textContent = "";
        return;
      }
      voiceLayer.removeAttribute("display");
      guide.removeAttribute("display");
      const input = Math.min(VOICE_CEIL, Math.max(VOICE_FLOOR, 20 * Math.log10(rms)));
      voiceSmooth += (input - voiceSmooth) * (input > voiceSmooth ? 0.5 : 0.2);
      voiceLevels[voiceWrite] = voiceSmooth;
      voiceWrite = (voiceWrite + 1) % VOICE_POINTS;
      voiceFilled = Math.min(voiceFilled + 1, VOICE_POINTS);
      for (let age = 0; age < VOICE_POINTS; age++) {
        const dot = voiceDots[age];
        if (age >= voiceFilled) {
          dot.setAttribute("display", "none");
          continue;
        }
        const level = voiceLevels[(voiceWrite - 1 - age + VOICE_POINTS) % VOICE_POINTS];
        dot.removeAttribute("display");
        dot.setAttribute("cx", xFor(level).toFixed(1));
        dot.setAttribute("cy", voiceY(level).toFixed(1));
        dot.setAttribute("r", (age === 0 ? 3.4 : Math.max(1, 2.6 - age * 0.03)).toFixed(2));
        dot.setAttribute("opacity",
          (age === 0 ? 1 : Math.max(0.06, 0.8 * (1 - age / VOICE_POINTS))).toFixed(2));
      }
      const x = xFor(voiceSmooth);
      const y = voiceY(voiceSmooth);
      const output = Math.min(24, Math.max(-60, compressedDb(
        voiceSmooth, curveSettings.threshold, curveSettings.ratio, curveSettings.knee)
        + curveSettings.makeup));
      guideIn.setAttribute("x1", x.toFixed(1));
      guideIn.setAttribute("y1", y.toFixed(1));
      guideIn.setAttribute("x2", x.toFixed(1));
      guideIn.setAttribute("y2", BOTTOM);
      guideOut.setAttribute("x1", LEFT);
      guideOut.setAttribute("y1", y.toFixed(1));
      guideOut.setAttribute("x2", x.toFixed(1));
      guideOut.setAttribute("y2", y.toFixed(1));
      voiceReadout.setAttribute("x", RIGHT - 4);
      voiceReadout.setAttribute("y", BOTTOM - 5);
      voiceReadout.textContent = `${voiceSmooth.toFixed(0)} in · ${output.toFixed(0)} out`;
    },
  };
  graph.update(settings);
  return graph;
}

export function effectGraph(key, settings, sampleRate) {
  if (key === "EQ") return eqGraph(settings, sampleRate);
  if (key === "Comp") return compressorGraph(settings);
  return null;
}
