<script>
  import Button from './Button.svelte';

  let { value = $bindable(''), busy = false, onsend, onstop, ...rest } = $props();
  let box = $state();

  function key(e) {
    if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      e.preventDefault();
      if (!busy && value.trim()) onsend();
    }
  }

  $effect(() => {
    value;
    box.style.setProperty('--height', 'auto');
    box.style.setProperty('--height', `${box.scrollHeight}px`);
  });
</script>

<form class="composer" onsubmit={(e) => (e.preventDefault(), busy ? onstop() : value.trim() && onsend())}>
  <textarea bind:this={box} bind:value rows="1" onkeydown={key} {...rest}></textarea>
  <Button icon={busy ? 'square' : 'arrow-up'} type="submit" title={busy ? 'Stop' : 'Send'} aria-label={busy ? 'Stop' : 'Send'} disabled={!busy && !value.trim()} />
</form>

<style>
  .composer {
    display: flex;
    align-items: flex-end;
    gap: var(--space-2);
    padding: var(--space-2);
    background: var(--panel);
    border-radius: var(--radius-xl);
    transition: background var(--dur) var(--ease);
  }
  .composer:focus-within { background: var(--raised); }
  textarea {
    flex: 1;
    min-width: 0;
    height: var(--height, auto);
    max-height: var(--tile);
    padding: var(--space-3);
    border: 0;
    background: none;
    color: var(--text);
    font: inherit;
    letter-spacing: inherit;
    resize: none;
    outline: 0;
  }
  textarea::placeholder { color: var(--text-3); }
  .composer :global(button:disabled) { opacity: var(--dim); }
</style>
