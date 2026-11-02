<script>
  import { Plus, ArrowUp, Square, Paperclip, Library, Globe, Telescope, Camera, Mic, AudioLines } from '@lucide/svelte';
  import { onDestroy } from 'svelte';
  import IconButton from './IconButton.svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import SubMenu from './SubMenu.svelte';
  import ModelPicker from './ModelPicker.svelte';
  import Attachment from './Attachment.svelte';
  import Chip from './Chip.svelte';
  import ConnectorIcon from './ConnectorIcon.svelte';
  import { Plug } from '@lucide/svelte';
  import { ws, openSettings, uploadFiles } from '../lib/app.svelte.js';
  import { rise } from '../lib/motion.js';

  let {
    value = $bindable(''),
    placeholder = 'How can I help you today?',
    size = 'md',
    streaming = false,
    model = $bindable('pro'),
    onsend,
    onstop,
    element = $bindable()
  } = $props();

  let attachments = $state([]);
  let web = $state(true);
  let research = $state(false);
  let thinking = $state(true);
  let focused = $state(false);

  const control = $derived(size === 'lg' ? 'md' : 'sm');
  const ready = $derived(value.trim().length > 0 || attachments.length > 0);

  function send() {
    if (streaming) {
      onstop?.();
      return;
    }
    if (!ready) return;
    onsend?.({ text: value.trim(), attachments: [...attachments], model, research });
    value = '';
    attachments = [];
  }

  function onkeydown(event) {
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
      event.preventDefault();
      send();
    }
  }

  function attach(file, real = false) {
    if (attachments.some((a) => a.id === file.id)) return;
    attachments.push({ id: file.id, name: file.name, kind: file.kind, size: file.size, tone: file.tone ?? 1, real });
  }

  // Files picked here go to the library first, then along with the message by id.
  let picker = $state();
  let uploading = $state(false);

  async function picked(event) {
    const list = [...event.currentTarget.files];
    event.currentTarget.value = '';
    if (!list.length) return;
    uploading = true;
    for (const f of await uploadFiles(list)) attach(f, true);
    uploading = false;
    element?.focus();
  }

  function attachSample(kind = 'image') {
    const n = attachments.length + 1;
    const file = kind === 'screenshot'
      ? { id: `up-${Date.now()}`, name: `Screenshot ${n}.png`, kind: 'image', size: '840 KB', tone: 3 }
      : { id: `up-${Date.now()}`, name: `scan-${String(n).padStart(3, '0')}.jpg`, kind: 'image', size: '2.2 MB', tone: (n % 4) + 1 };
    attach(file);
    element?.focus();
  }

  // a real recording, kept as an attachment you can play before sending
  let recorder = $state(null);
  let seconds = $state(0);
  let wave = $state();
  let clock, audio, frame;
  let levels = [];
  let sampled = 0;
  const STEP = 70;
  const elapsed = $derived(`${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`);

  async function record() {
    let stream;
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    } catch {
      return;
    }
    const chunks = [];
    const r = new MediaRecorder(stream);
    audio = new AudioContext();
    const analyser = audio.createAnalyser();
    analyser.fftSize = 1024;
    audio.createMediaStreamSource(stream).connect(analyser);
    levels = [];
    draw(analyser);
    r.ondataavailable = (e) => e.data.size && chunks.push(e.data);
    r.onstop = () => {
      stream.getTracks().forEach((t) => t.stop());
      const blob = new Blob(chunks, { type: r.mimeType || 'audio/webm' });
      const kb = blob.size / 1024;
      attachments.push({ id: `rec-${Date.now()}`, name: `Recording ${elapsed}`, kind: 'recording', size: kb < 1024 ? `${Math.max(1, Math.round(kb))} KB` : `${(kb / 1024).toFixed(1)} MB`, tone: 2, href: URL.createObjectURL(blob) });
      clearInterval(clock);
      cancelAnimationFrame(frame);
      audio?.close();
      audio = null;
      recorder = null;
      element?.focus();
    };
    seconds = 0;
    clock = setInterval(() => seconds++, 1000);
    r.start(1000);
    recorder = r;
  }

  // the voice as it comes in, as Voice Memos draws it: one thin bar per moment, newest on the right,
  // scrolling left as you talk and fading out at the far edge
  function draw(analyser) {
    frame = requestAnimationFrame(() => draw(analyser));
    const now = performance.now();
    if (now - sampled >= STEP) {
      const data = new Float32Array(analyser.fftSize);
      analyser.getFloatTimeDomainData(data);
      let sum = 0;
      for (const v of data) sum += v * v;
      levels.push(Math.min(1, Math.sqrt(Math.sqrt(sum / data.length)) * 1.6));
      if (levels.length > 400) levels.shift();
      sampled = now;
    }
    if (!wave) return;
    const { width, height } = wave.getBoundingClientRect();
    const scale = devicePixelRatio;
    if (wave.width !== Math.round(width * scale)) wave.width = Math.round(width * scale);
    if (wave.height !== Math.round(height * scale)) wave.height = Math.round(height * scale);
    const c = wave.getContext('2d');
    c.clearRect(0, 0, wave.width, wave.height);
    c.fillStyle = getComputedStyle(wave).color;
    const bar = 2 * scale;
    const pitch = 5 * scale;
    const slide = Math.min(1, (now - sampled) / STEP) * pitch;
    for (let i = 0; i < levels.length; i++) {
      const x = wave.width - (levels.length - i) * pitch - slide + pitch;
      if (x < -bar) continue;
      const h = Math.max(bar, levels[i] * wave.height * 0.9);
      c.beginPath();
      c.roundRect(x, (wave.height - h) / 2, bar, h, bar / 2);
      c.fill();
    }
  }

  onDestroy(() => {
    clearInterval(clock);
    cancelAnimationFrame(frame);
    audio?.close();
    audio = null;
    if (recorder?.state === 'recording') recorder.stop();
  });

  function dictate() {
    value = (value ? value + ' ' : '') + 'What did Grandma write about the harvest in 1974?';
    element?.focus();
  }

</script>

<form class={['composer', size, { focused }]} onsubmit={(e) => { e.preventDefault(); send(); }}>
  {#if attachments.length}
    <div class="atts">
      {#each attachments as a (a.id)}
        <span in:rise={{ shift: '--shift-sm' }}>
          <Attachment name={a.name} kind={a.kind} size={a.size} tone={a.tone} href={a.href} onremove={() => (attachments = attachments.filter((x) => x.id !== a.id))} />
        </span>
      {/each}
    </div>
  {/if}
  <input class="picker" type="file" multiple bind:this={picker} onchange={picked} tabindex="-1" aria-hidden="true" />
  <textarea
    bind:this={element}
    bind:value
    rows="1"
    {placeholder}
    aria-label="Message"
    {onkeydown}
    onfocus={() => (focused = true)}
    onblur={() => (focused = false)}
  ></textarea>
  <div class="bar">
    <Menu placement={size === 'lg' ? 'bottom-start' : 'top-start'} label="Add">
      {#snippet trigger(props)}
        <IconButton icon={Plus} label="Add files and more" variant="secondary" size={control} {...props} />
      {/snippet}
      <MenuItem icon={Paperclip} onselect={() => picker?.click()}>{uploading ? 'Uploading…' : 'Add files or photos'}</MenuItem>
      <MenuItem icon={Camera} onselect={() => attachSample('screenshot')}>Take a screenshot</MenuItem>
      {#if ws().library.length}
        <SubMenu icon={Library} label="Add from library">
          {#each ws().library.slice(0, 8) as file (file.id)}
            <MenuItem checked={attachments.some((a) => a.id === file.id) || undefined} onselect={() => attach(file, true)}>{file.name}</MenuItem>
          {/each}
        </SubMenu>
      {/if}
      {#if ws().connectors.some((c) => c.connected)}
        <SubMenu icon={Plug} label="Add from a connector">
          {#each ws().connectors.filter((c) => c.connected) as c (c.id)}
            <MenuItem onselect={() => attach({ id: `${c.id}-${Date.now()}`, name: `From ${c.name}`, kind: 'document', size: '1.1 MB', tone: 3 })}>
              {#snippet leading()}<ConnectorIcon id={c.id} size="xs" />{/snippet}
              {c.name}
            </MenuItem>
          {/each}
          <MenuSeparator />
          <MenuItem icon={Plug} onselect={() => openSettings('connectors')}>Manage connectors</MenuItem>
        </SubMenu>
      {:else}
        <MenuItem icon={Plug} onselect={() => openSettings('connectors')}>Connect an app</MenuItem>
      {/if}
      <MenuSeparator />
      <MenuItem icon={Telescope} keepOpen toggle={research} onselect={() => (research = !research)}>Research</MenuItem>
      <MenuItem icon={Globe} keepOpen toggle={web} onselect={() => (web = !web)}>Web search</MenuItem>
    </Menu>
    {#if research}
      <span in:rise={{ shift: '--shift-sm' }}><Chip icon={Telescope} size="sm" onremove={() => (research = false)}>Research</Chip></span>
    {/if}
    {#if recorder}
      <canvas class="wave" bind:this={wave} aria-hidden="true"></canvas>
    {:else}
      <span class="spacer"></span>
      <ModelPicker bind:model bind:thinking size={control} />
    {/if}
    {#if recorder}
      <span class="elapsed" aria-live="polite"><span class="dot"></span>{elapsed}</span>
      <IconButton icon={Square} label="Stop recording" variant="inverse" round size={control} onclick={() => recorder.stop()} />
    {:else if streaming}
      <IconButton icon={Square} label="Stop" variant="inverse" round size={control} type="submit" />
    {:else}
      <IconButton icon={AudioLines} label="Record" variant="ghost" round size={control} onclick={record} />
      {#if size === 'lg' && !ready}
        <IconButton icon={Mic} label="Dictate" variant="ghost" round size={control} onclick={dictate} />
      {:else}
        <IconButton icon={ArrowUp} label="Send" variant="primary" round size={control} disabled={!ready} type="submit" shortcut="↵" />
      {/if}
    {/if}
  </div>
</form>

<style>
  .picker {
    display: none;
  }

  .composer {
    display: flex;
    flex-direction: column;
    gap: var(--composer-gap);
    width: 100%;
    padding: var(--composer-pad);
    border-radius: var(--composer-radius);
    background: var(--color-surface);
    box-shadow: var(--ring);
    cursor: text;
    transition: box-shadow var(--duration-normal) var(--ease-out);
  }

  .focused {
    box-shadow: var(--ring-strong);
  }

  .atts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    padding: var(--space-1);
  }

  textarea {
    display: block;
    width: 100%;
    min-height: var(--composer-line);
    max-height: var(--composer-max);
    padding: var(--space-1);
    field-sizing: content;
    resize: none;
    color: var(--color-text);
    font-size: var(--composer-text);
    line-height: var(--leading-normal);
    letter-spacing: var(--tracking-normal);
  }

  .md {
    --composer-pad: var(--space-2);
    --composer-gap: var(--space-1);
    --composer-radius: var(--radius-xl);
    --composer-text: var(--text-md);
    --composer-line: var(--control-sm);
  }

  .lg {
    --composer-pad: var(--space-3);
    --composer-gap: var(--space-2);
    --composer-radius: var(--radius-2xl);
    --composer-text: var(--text-base);
    --composer-line: var(--control-2xl);
  }

  textarea::placeholder {
    color: var(--color-text-tertiary);
  }

  textarea:focus-visible {
    outline: none;
  }

  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }

  .spacer {
    flex: 1;
  }

  .wave {
    flex: 1;
    min-width: 0;
    height: var(--control-sm);
    color: var(--color-text);
    mask-image: linear-gradient(to right, transparent, black 30%);
  }

  .elapsed {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    padding-inline: var(--space-1);
    color: var(--color-text-secondary);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
  }

  .dot {
    width: var(--space-2);
    height: var(--space-2);
    border-radius: 50%;
    background: var(--color-danger);
    animation: beat 1.4s ease-in-out infinite;
  }

  @keyframes beat {
    50% { opacity: 0.25; }
  }
</style>
