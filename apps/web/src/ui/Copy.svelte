<script>
  import Icon from '#kit/Icon.svelte';

  let { text, label = 'Copy' } = $props();
  let done = $state(false);
  let timer;

  async function copy() {
    await navigator.clipboard.writeText(typeof text === 'function' ? text() : text);
    done = true;
    clearTimeout(timer);
    timer = setTimeout(() => (done = false), 1400);
  }
</script>

<button class="copy" class:done type="button" title={done ? 'Copied' : label} aria-label={done ? 'Copied' : label} onclick={copy}>
  <Icon name={done ? 'check' : 'copy'} />
</button>

<style>
  .copy {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--control-height-sm);
    height: var(--control-height-sm);
    border-radius: var(--radius-item);
    color: var(--fg-muted);
    transition: background var(--duration-fast), color var(--duration-fast);
  }
  .copy:hover { background: var(--fill-faint); color: var(--fg); }
  .done { color: var(--fg); }
</style>
