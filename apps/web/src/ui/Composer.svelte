<script>
  import Icon from '#kit/Icon.svelte';

  let { value = $bindable(''), busy = false, onsend, onstop, field = $bindable(), lead, tail, ontalk, ...rest } = $props();

  const ready = $derived(value.trim() !== '');

  function key(e) {
    if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      e.preventDefault();
      if (!busy && ready) onsend(value);
    }
  }

  function submit(e) {
    e.preventDefault();
    if (busy) onstop();
    else if (ready) onsend(value);
  }
</script>

<form class="composer" onsubmit={submit}>
  {@render lead?.()}
  <div class="grow" data-value={value}>
    <textarea bind:this={field} bind:value rows="1" autocapitalize="sentences" enterkeyhint="send" onkeydown={key} {...rest}></textarea>
  </div>
  {@render tail?.()}
  {#if !busy && !ready && ontalk}
    <button class="send" type="button" title="Record" aria-label="Record" onclick={ontalk}><Icon name="audio-lines" /></button>
  {:else}
    <button class="send" class:busy type="submit" disabled={!busy && !ready} title={busy ? 'Stop' : 'Send'} aria-label={busy ? 'Stop' : 'Send'}>
      <Icon name={busy ? 'square' : 'arrow-up'} />
    </button>
  {/if}
</form>

<style>
  .composer {
    display: flex;
    align-items: flex-end;
    gap: var(--space-1);
    padding: var(--space-1-5);
    border-radius: var(--radius-md);
    background: var(--composer-bg);
  }
  .grow { display: grid; flex: 1; min-width: 0; max-height: var(--composer-max); overflow-y: auto; }
  .grow::after { content: attr(data-value) ' '; visibility: hidden; white-space: pre-wrap; }
  textarea, .grow::after {
    grid-area: 1 / 1;
    min-width: 0;
    padding: var(--composer-pad);
    font: var(--text-chat) / var(--leading-medium) var(--font-sans);
    letter-spacing: inherit;
    overflow-wrap: anywhere;
  }
  textarea { overflow: hidden; border: 0; background: none; color: var(--fg); resize: none; outline: 0; }
  textarea::placeholder { color: var(--fg-muted); }
  .send {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--composer-line);
    height: var(--composer-line);
    border-radius: var(--radius-full);
    background: var(--primary);
    color: var(--on-primary);
    transition: background var(--duration-fast), transform var(--duration-fast) var(--ease-out);
  }
  .send:hover { background: var(--primary-hover); }
  .send:active { transform: scale(var(--scale-press)); }
  .busy :global(.icon) { fill: currentColor; }
</style>
