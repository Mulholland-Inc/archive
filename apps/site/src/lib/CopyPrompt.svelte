<script>
  let { text, label = 'Copy self-host prompt' } = $props();

  let copied = $state(false);

  async function copy() {
    await navigator.clipboard.writeText(text);
    copied = true;
    setTimeout(() => (copied = false), 1500);
  }
</script>

<button type="button" class="btn btn-ghost" onclick={copy}><span aria-live="polite">{copied ? 'Copied' : label}</span></button>
