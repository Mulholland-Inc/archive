<script>
  import { onDestroy } from 'svelte';
  import Icon from '#kit/Icon.svelte';

  let { tint = '#ffffff', kind = 'family', onsave, onclose } = $props();
  let state = $state('starting');
  let seconds = $state(0);
  let problem = $state('');
  let canvas = $state();
  let inputs = $state([]);
  let input = $state('');
  let stream, recorder, audio, analyser, source, out, timer, frame;
  let chunks = [];
  const noun = kind === 'business' ? 'memo' : 'memory';

  const clock = $derived(`${String(Math.floor(seconds / 60)).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}`);
  const recording = $derived(state === 'recording');

  function draw() {
    frame = requestAnimationFrame(draw);
    if (!canvas || !analyser) return;
    const c = canvas.getContext('2d');
    const { width, height } = canvas;
    const data = new Uint8Array(analyser.frequencyBinCount);
    analyser.getByteFrequencyData(data);
    c.clearRect(0, 0, width, height);
    c.fillStyle = recording ? tint : '#3a3d45';
    const bars = 56;
    const gap = width / bars;
    for (let i = 0; i < bars; i++) {
      const mirror = Math.abs(i - (bars - 1) / 2) / (bars / 2);
      const v = recording ? data[Math.floor(mirror * data.length * 0.45)] / 255 : 0;
      const h = Math.max(6, v * height * 0.95);
      c.beginPath();
      c.roundRect(i * gap + gap * 0.3, (height - h) / 2, gap * 0.4, h, gap * 0.2);
      c.fill();
    }
  }

  async function open(id) {
    const next = await navigator.mediaDevices.getUserMedia({ audio: id ? { deviceId: { exact: id } } : true });
    source?.disconnect();
    stream?.getTracks().forEach((t) => t.stop());
    stream = next;
    source = audio.createMediaStreamSource(stream);
    source.connect(analyser);
    source.connect(out);
  }

  async function start() {
    audio = new AudioContext();
    analyser = audio.createAnalyser();
    analyser.fftSize = 512;
    analyser.smoothingTimeConstant = 0.72;
    out = audio.createMediaStreamDestination();
    try {
      await open('');
    } catch {
      state = 'failed';
      problem = 'Allow the microphone in your browser, then try again.';
      return;
    }
    const devices = await navigator.mediaDevices.enumerateDevices();
    inputs = devices.filter((d) => d.kind === 'audioinput' && d.deviceId !== 'default' && d.deviceId !== 'communications');
    // Chrome calls the active microphone "default", which is not in the list.
    const active = stream.getAudioTracks()[0];
    input = (inputs.find((d) => d.deviceId === active?.getSettings().deviceId || d.label === active?.label) ?? inputs[0])?.deviceId ?? '';
    recorder = new MediaRecorder(out.stream);
    recorder.ondataavailable = (e) => e.data.size && chunks.push(e.data);
    recorder.start(1000);
    state = 'recording';
    timer = setInterval(() => state === 'recording' && seconds++, 1000);
    draw();
  }

  function pause() {
    if (recording) {
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
      if (keep) onsave(new File(chunks, `${noun === 'memo' ? 'Memo' : 'Memory'} ${when}.${type.includes('mp4') ? 'm4a' : 'webm'}`, { type }));
      onclose();
    };
    recorder.stop();
  }

  onDestroy(() => {
    clearInterval(timer);
    cancelAnimationFrame(frame);
    stream?.getTracks().forEach((t) => t.stop());
    audio?.close();
  });

  start();
</script>

<section class="recorder" aria-label="Recording a {noun}">
  <header>
    <button class="ghost" type="button" onclick={() => finish(false)}><Icon name="x" />Discard</button>
    {#if inputs.length}
      <label class="input">
        <Icon name="mic" />
        <select bind:value={input} aria-label="Microphone" onchange={() => open(input)}>
          {#each inputs as d, i (d.deviceId)}<option value={d.deviceId}>{d.label || `Microphone ${i + 1}`}</option>{/each}
        </select>
        <Icon name="chevron-down" />
      </label>
    {/if}
  </header>

  {#if state === 'failed'}
    <p class="problem">{problem}</p>
  {:else}
    <div class="stage">
      <div class="status" aria-live="polite"><span class="dot" class:still={!recording}></span>{state === 'paused' ? 'Paused' : state === 'starting' ? 'Getting ready' : 'Recording'}</div>
      <div class="time">{clock}</div>
      <canvas bind:this={canvas} width="1120" height="240" aria-hidden="true"></canvas>
    </div>
    <footer>
      <button class="round" type="button" disabled={state === 'starting'} aria-label={recording ? 'Pause' : 'Keep recording'} title={recording ? 'Pause' : 'Keep recording'} onclick={pause}><Icon name={recording ? 'pause' : 'mic'} /></button>
      <button class="save" type="button" disabled={state === 'starting'} style="--tint: {tint}" onclick={() => finish(true)}><Icon name="check" />Save {noun}</button>
    </footer>
  {/if}
</section>

<style>
  .recorder { position: absolute; inset: var(--space-2); z-index: calc(var(--z-raised) + 2); display: flex; flex-direction: column; overflow: hidden; border-radius: var(--radius-panel); background: #08090c; color: #fff; animation: open 220ms var(--ease-out) both; }
  header { display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); padding: var(--space-3); }
  .ghost { display: inline-flex; align-items: center; gap: var(--space-1); color: #8f95a3; transition: color var(--duration-fast); }
  .ghost:hover { color: #fff; }
  .ghost :global(.icon), .input :global(.icon) { width: 15px; height: 15px; }
  .input { position: relative; display: inline-flex; align-items: center; gap: var(--space-1-5); max-width: 62%; padding: var(--space-1) var(--space-2); border-radius: var(--radius-full); background: #17191f; color: #c9cdd6; font-size: var(--text-table); }
  .input select { min-width: 0; max-width: 240px; appearance: none; border: 0; background: none; color: inherit; font: inherit; text-overflow: ellipsis; outline: 0; cursor: pointer; }
  .stage { display: flex; flex: 1; flex-direction: column; align-items: center; justify-content: center; gap: var(--space-5); min-height: 0; padding: var(--space-3); }
  .status { display: flex; align-items: center; gap: var(--space-2); color: #8f95a3; font-size: var(--text-table); }
  .dot { width: 9px; height: 9px; border-radius: 50%; background: #ff3b30; animation: beat 1.4s ease-in-out infinite; }
  .dot.still { animation: none; background: #4a4e59; }
  .time { font: var(--weight-medium) clamp(48px, 9vw, 76px) / 1 var(--font-serif); font-variant-numeric: tabular-nums; letter-spacing: var(--tracking-tight); }
  canvas { width: min(560px, 100%); height: 120px; }
  footer { display: flex; align-items: center; justify-content: center; gap: var(--space-3); padding: var(--space-5) var(--space-3) var(--space-6); }
  .round { display: grid; place-items: center; width: 52px; height: 52px; border-radius: 50%; background: #17191f; color: #fff; transition: background var(--duration-fast); }
  .round:hover { background: #24272f; }
  .round :global(.icon) { width: 20px; height: 20px; }
  .save { display: inline-flex; align-items: center; gap: var(--space-2); height: 52px; padding: 0 var(--space-5); border-radius: var(--radius-full); background: #fff; color: #08090c; font-weight: var(--weight-medium); transition: transform var(--duration-fast) var(--ease-out); }
  .save:hover { transform: translateY(-1px); }
  .save :global(.icon) { width: 18px; height: 18px; color: var(--tint); }
  .problem { margin: auto; padding: var(--space-4); color: #c9cdd6; text-align: center; }
  @keyframes beat { 50% { opacity: 0.25; } }
  @keyframes open { from { opacity: 0; transform: scale(0.985); } }
</style>
