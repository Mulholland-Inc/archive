<script>
  import Button from './Button.svelte';
  import { clipboard } from './copy.svelte.js';

  let { text, label = 'Copy self-host prompt' } = $props();

  const clip = clipboard();
</script>

<Button ghost onclick={() => clip.copy(text)}><span aria-live="polite">{clip.copied ? 'Copied' : label}</span></Button>
