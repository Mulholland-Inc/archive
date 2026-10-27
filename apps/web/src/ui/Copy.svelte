<script>
  import Button from './Button.svelte';

  let { text } = $props();
  let done = $state(false);

  async function copy() {
    await navigator.clipboard.writeText(text);
    done = true;
    setTimeout(() => (done = false), 1500);
  }
</script>

<div class="copy">
  <code>{text}</code>
  <Button quiet icon={done ? 'check' : 'copy'} title="Copy" aria-label="Copy" onclick={copy} />
</div>

<style>
  .copy { display: flex; align-items: center; gap: var(--space-1); padding: var(--space-1); background: var(--raised); border-radius: var(--radius-pill); }
  code { flex: 1; min-width: 0; padding: var(--space-2); overflow-x: auto; white-space: nowrap; font: var(--text-xs) var(--mono); color: var(--text-2); }
</style>
