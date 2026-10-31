import { ui } from './looks.svelte.js';

let context;

function audio() {
  if (!ui.sounds) return null;
  try {
    context ??= new AudioContext();
    return context;
  } catch {
    return null;
  }
}

function burst(sound, seconds, make) {
  const frames = Math.floor(sound.sampleRate * seconds);
  const buffer = sound.createBuffer(1, frames, sound.sampleRate);
  const data = buffer.getChannelData(0);
  for (let i = 0; i < frames; i++) data[i] = make(i / frames);
  const source = sound.createBufferSource();
  source.buffer = buffer;
  return source;
}

// A capsule rushing down the pneumatic tube.
export function whoosh() {
  const sound = audio();
  if (!sound) return;
  const air = burst(sound, 0.42, (t) => (Math.random() * 2 - 1) * Math.sin(Math.PI * t));
  const filter = sound.createBiquadFilter();
  filter.type = 'bandpass';
  filter.Q.value = 1.4;
  filter.frequency.setValueAtTime(320, sound.currentTime);
  filter.frequency.exponentialRampToValueAtTime(2600, sound.currentTime + 0.34);
  const gain = sound.createGain();
  gain.gain.value = 0.07;
  air.connect(filter).connect(gain).connect(sound.destination);
  air.start();
}

// One key of the teleprinter.
export function clack() {
  const sound = audio();
  if (!sound) return;
  const key = burst(sound, 0.018, (t) => (Math.random() * 2 - 1) * (1 - t) ** 3);
  const filter = sound.createBiquadFilter();
  filter.type = 'highpass';
  filter.frequency.value = 1500 + Math.random() * 900;
  const gain = sound.createGain();
  gain.gain.value = 0.05;
  key.connect(filter).connect(gain).connect(sound.destination);
  key.start();
}

// The typewriter bell at the end of the line.
export function ding() {
  const sound = audio();
  if (!sound) return;
  for (const [frequency, level] of [[2093, 0.05], [5230, 0.016]]) {
    const bell = sound.createOscillator();
    const gain = sound.createGain();
    bell.frequency.value = frequency;
    gain.gain.setValueAtTime(level, sound.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.0001, sound.currentTime + 0.9);
    bell.connect(gain).connect(sound.destination);
    bell.start();
    bell.stop(sound.currentTime + 0.95);
  }
}
