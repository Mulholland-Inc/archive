<script>
  import Icon from '#kit/Icon.svelte';

  let { onremove, label = '', children } = $props();
</script>

<span class="chip">{@render children()}{#if onremove}<button type="button" aria-label="Remove {label}" onclick={onremove}><Icon name="x" /></button>{/if}</span>

<style>
  .chip {
    display: inline-flex;
    flex: none;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-1);
    border-radius: var(--radius-xs);
    background: var(--fill);
    color: var(--fg-muted);
    font-size: var(--text-meta);
    line-height: var(--leading-none);
    white-space: nowrap;
  }
  button { display: grid; place-items: center; color: inherit; transition: color var(--duration-fast); }
  button:hover { color: var(--fg); }
  button :global(.icon) { width: var(--text-meta); height: var(--text-meta); }
</style>
