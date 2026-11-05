<script>
  import { ArrowUp, Square, Paperclip, Mic } from '@lucide/svelte';
  import IconButton from './IconButton.svelte';
  import Attachment from './Attachment.svelte';
  import { uploadFiles, ws } from '../lib/app.svelte.js';
  import { api, loadDraft, saveDraft } from '../lib/api.js';
  import { notify } from '../lib/dialogs.svelte.js';
  import { rise } from '../lib/motion.js';

  let {
    value = $bindable(''),
    placeholder = 'How can I help you today?',
    size = 'md',
    streaming = false,
    onsend,
    onstop,
    element = $bindable(),
    draft
  } = $props();

  let loaded = $state();

  $effect(() => {
    const key = draft;
    if (!key) return;
    loadDraft(key).then((text) => {
      if (draft !== key) return;
      value = text;
      loaded = key;
    });
  });

  $effect(() => {
    if (draft && loaded === draft) saveDraft(draft, value);
  });

  let attachments = $state([]);
  let focused = $state(false);

  const control = $derived(size === 'lg' ? 'md' : 'sm');
  const speech = $derived(ws().catalog?.catalog.speech && navigator.mediaDevices && window.MediaRecorder);

  let dictation = $state();
  let recording = $state(false);

  const ready = $derived(!dictation && (value.trim().length > 0 || attachments.length > 0));

  function end() {
    dictation?.abort();
    dictation = null;
    recording = false;
  }

  $effect(() => {
    draft;
    return end;
  });

  async function dictate() {
    if (recording) {
      recording = false;
      dictation.recorder.stop();
      return;
    }
    const ctl = (dictation = new AbortController());
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const release = () => stream.getTracks().forEach((t) => t.stop());
      if (ctl.signal.aborted) return release();
      const recorder = new MediaRecorder(stream);
      const chunks = [];
      recorder.ondataavailable = (e) => chunks.push(e.data);
      ctl.signal.addEventListener('abort', () => {
        recorder.onstop = null;
        if (recorder.state !== 'inactive') recorder.stop();
        release();
      });
      recorder.onstop = () => {
        release();
        transcribe(new Blob(chunks, { type: recorder.mimeType }), ctl);
      };
      ctl.recorder = recorder;
      recorder.start();
      recording = true;
    } catch (error) {
      fail(error, ctl);
    }
  }

  async function transcribe(audio, ctl) {
    try {
      const { text } = await api('/api/speech', { method: 'POST', raw: audio, headers: { 'Content-Type': audio.type }, signal: ctl.signal });
      if (text) value = value.trim() ? `${value.trimEnd()} ${text}` : text;
      dictation = null;
      element?.focus();
    } catch (error) {
      fail(error, ctl);
    }
  }

  function fail(error, ctl) {
    if (ctl.signal.aborted) return;
    end();
    notify({ title: 'That didn’t work', body: error.message });
  }

  function send() {
    if (streaming) {
      onstop?.();
      return;
    }
    if (!ready) return;
    onsend?.({ text: value.trim(), attachments: [...attachments] });
    value = '';
    attachments = [];
    if (draft) saveDraft(draft, '');
  }

  function onkeydown(event) {
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
      event.preventDefault();
      send();
    }
  }

  function onmousedown(event) {
    if (event.target === element) return;
    event.preventDefault();
    element?.focus();
  }

  let picker = $state();
  let uploading = $state(false);

  async function picked(event) {
    const list = [...event.currentTarget.files];
    event.currentTarget.value = '';
    if (!list.length) return;
    uploading = true;
    for (const { id, name, kind, size, tone = 1 } of await uploadFiles(list)) if (!attachments.some((a) => a.id === id)) attachments.push({ id, name, kind, size, tone });
    uploading = false;
    element?.focus();
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<form class={['composer', size, { focused }]} {onmousedown} onsubmit={(e) => { e.preventDefault(); send(); }}>
  {#if attachments.length}
    <div class="atts">
      {#each attachments as a (a.id)}
        <span in:rise={{ shift: '--shift-sm' }}>
          <Attachment name={a.name} kind={a.kind} size={a.size} onremove={() => (attachments = attachments.filter((x) => x.id !== a.id))} />
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
    <IconButton icon={Paperclip} label="Attach files" variant="ghost" size={control} disabled={uploading} onclick={() => picker?.click()} />
    <span class="spacer"></span>
    {#if speech}
      <IconButton icon={recording ? Square : Mic} label={recording ? 'Stop' : 'Dictate'} variant="ghost" size={control} pressed={recording} disabled={dictation && !recording} onclick={dictate} />
    {/if}
    {#if streaming}
      <IconButton icon={Square} label="Stop" variant="inverse" round size={control} type="submit" />
    {:else}
      <IconButton icon={ArrowUp} label="Send" variant="primary" round size={control} disabled={!ready} type="submit" shortcut="↵" />
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

  .bar :global(button:disabled) {
    pointer-events: none;
  }

  .spacer {
    flex: 1;
  }
</style>
