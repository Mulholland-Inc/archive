<script>
  import { onDestroy } from 'svelte';
  import Icon from '#kit/Icon.svelte';

  let { value = $bindable('') } = $props();
  let on = $state(false);
  let hint = $state('');
  let talk;

  const Speech = globalThis.SpeechRecognition ?? globalThis.webkitSpeechRecognition;

  function toggle() {
    if (on) return talk?.stop();
    if (!Speech) return (hint = 'Dictation needs Chrome or Safari');
    const base = value ? `${value.trimEnd()} ` : '';
    talk = new Speech();
    talk.continuous = true;
    talk.interimResults = true;
    talk.onresult = (e) => (value = base + [...e.results].map((r) => r[0].transcript).join(''));
    talk.onend = () => (on = false);
    talk.onerror = () => {
      on = false;
      hint = 'Allow the microphone in your browser';
    };
    hint = '';
    on = true;
    talk.start();
  }

  onDestroy(() => talk?.abort());
</script>

<button class="dictate" class:on type="button" aria-label={on ? 'Stop dictating' : 'Dictate'} aria-pressed={on} title={hint || (on ? 'Stop dictating' : 'Dictate')} onclick={toggle}><Icon name={on ? 'mic-off' : 'mic'} /></button>

<style>
  .dictate { display: grid; place-items: center; flex: none; align-self: flex-end; width: var(--composer-line); height: var(--composer-line); border-radius: var(--radius-full); color: var(--fg-muted); transition: background var(--duration-fast), color var(--duration-fast); }
  .dictate:hover { background: var(--fill); color: var(--fg); }
  .dictate.on { background: #ff3b30; color: #fff; }
</style>
