<script>
  import { onDestroy } from 'svelte';
  import Icon from '#kit/Icon.svelte';

  let { onsave, onclose } = $props();
  let state = $state('starting');
  let seconds = $state(0);
  let problem = $state('');
  let canvas = $state();
  let stream, recorder, audio, analyser, timer, frame;
  let chunks = [];

  const clock = $derived(`${String(Math.floor(seconds / 60)).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}`);

  function draw() {
    frame = requestAnimationFrame(draw);
    if (!canvas || !analyser) return;
    const c = canvas.getContext('2d');
    const { width, height } = canvas;
    const data = new Uint8Array(analyser.frequencyBinCount);
    analyser.getByteTimeDomainData(data);
    c.clearRect(0, 0, width, height);
    c.fillStyle = state === 'paused' ? '#555a66' : '#ffffff';
    const bars = 48;
    const gap = width / bars;
    for (let i = 0; i < bars; i++) {
      const v = state === 'paused' ? 0 : Math.abs(data[Math.floor((i / bars) * data.length)] - 128) / 128;
      const h = Math.max(4, Math.min(1, v * 3.2) * height * 0.92);
      c.beginPath();
      c.roundRect(i * gap + gap * 0.28, (height - h) / 2, gap * 0.44, h, gap * 0.22);
      c.fill();
    }
  }

  async function start() {
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    } catch {
      state = 'failed';
      problem = 'Your browser needs permission to use the microphone.';
      return;
    }
    recorder = new MediaRecorder(stream);
    recorder.ondataavailable = (e) => e.data.size && chunks.push(e.data);
    audio = new AudioContext();
    analyser = audio.createAnalyser();
    analyser.fftSize = 1024;
    audio.createMediaStreamSource(stream).connect(analyser);
    recorder.start(1000);
    state = 'recording';
    timer = setInterval(() => state === 'recording' && seconds++, 1000);
    draw();
  }

  function pause() {
    if (state === 'recording') {
      recorder.pause();
      state = 'paused';
    } else {
      recorder.resume();
      state = 'recording';
    }
  }

  function finish(keep) {
    if (!recorder || recorder.state === 'inactive') return onclose();
    recorder.onstop = () => {
      const type = recorder.mimeType || 'audio/webm';
      const when = new Date().toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' }).replaceAll(':', '.').replaceAll('/', '-');
      if (keep) onsave(new File(chunks, `Memory ${when}.${type.includes('mp4') ? 'm4a' : 'webm'}`, { type }));
      onclose();
    };
    recorder.stop();
  }

  function cleanup() {
    clearInterval(timer);
    cancelAnimationFrame(frame);
    stream?.getTracks().forEach((t) => t.stop());
    audio?.close();
  }

  onDestroy(cleanup);
  start();
</script>

<div class="recorder" role="dialog" aria-modal="true" aria-label="Recording a memory">
  <button class="cancel" type="button" onclick={() => finish(false)}>Cancel</button>
  <div class="status" aria-live="polite">
    {#if state === 'failed'}
      <p>{problem}</p>
    {:else}
      <span class="dot" class:still={state !== 'recording'}></span>
      <span>{state === 'paused' ? 'Paused' : state === 'starting' ? 'Starting' : 'Recording'}</span>
    {/if}
  </div>
  {#if state !== 'failed'}
    <div class="time">{clock}</div>
    <canvas bind:this={canvas} width="960" height="240" aria-hidden="true"></canvas>
    <div class="controls">
      <button class="round" type="button" disabled={state === 'starting'} aria-label={state === 'paused' ? 'Keep recording' : 'Pause'} title={state === 'paused' ? 'Keep recording' : 'Pause'} onclick={pause}><Icon name={state === 'paused' ? 'mic' : 'pause'} /></button>
      <button class="stop" type="button" disabled={state === 'starting'} aria-label="Stop and save" title="Stop and save" onclick={() => finish(true)}><span></span></button>
    </div>
    <p class="hint">Stop saves it to this archive.</p>
  {/if}
</div>

<style>
  .recorder { position: fixed; inset: 0; z-index: calc(var(--z-nav) + 10); display: flex; flex-direction: column; align-items: center; justify-content: center; gap: var(--space-6); padding: var(--space-5); background: #000; color: #fff; }
  .cancel { position: absolute; top: var(--space-4); left: var(--space-4); color: #9aa0ad; }
  .cancel:hover { color: #fff; }
  .status { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-lg); }
  .dot { width: 12px; height: 12px; border-radius: 50%; background: #ff3b30; animation: beat 1.2s ease-in-out infinite; }
  .dot.still { animation: none; background: #555a66; }
  .time { font: var(--weight-medium) clamp(56px, 14vw, 104px) / 1 var(--font-sans); font-variant-numeric: tabular-nums; letter-spacing: var(--tracking-tight); }
  canvas { width: min(760px, 100%); height: 160px; }
  .controls { display: flex; align-items: center; gap: var(--space-6); }
  .round { display: grid; place-items: center; width: 64px; height: 64px; border-radius: 50%; background: #1f2126; color: #fff; }
  .round:hover { background: #2c2f36; }
  .round :global(.icon) { width: 26px; height: 26px; }
  .stop { display: grid; place-items: center; width: 88px; height: 88px; border: 4px solid #fff; border-radius: 50%; }
  .stop span { width: 34px; height: 34px; border-radius: 8px; background: #ff3b30; }
  .hint { color: #9aa0ad; font-size: var(--text-meta); }
  @keyframes beat { 50% { opacity: 0.25; } }
</style>
