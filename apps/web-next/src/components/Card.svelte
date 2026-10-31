<script>
  let { href, onclick, padding = 'md', variant = 'outline', full = false, children, ...rest } = $props();
  const interactive = $derived(Boolean(href || onclick));
</script>

{#if href}
  <a {href} class={['card', `p-${padding}`, variant, 'interactive', { full }]} {...rest}>{@render children()}</a>
{:else if onclick}
  <button type="button" class={['card', `p-${padding}`, variant, 'interactive', { full }]} {onclick} {...rest}>{@render children()}</button>
{:else}
  <div class={['card', `p-${padding}`, variant, { full, interactive }]} {...rest}>{@render children()}</div>
{/if}

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    min-width: 0;
    border-radius: var(--radius-xl);
    text-align: start;
    transition:
      box-shadow var(--duration-normal) var(--ease-out),
      background-color var(--duration-normal) var(--ease-out),
      transform var(--duration-normal) var(--ease-out);
  }

  .outline {
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
  }

  .filled {
    background: var(--color-fill-subtle);
  }

  .raised {
    background: var(--color-surface);
    box-shadow: var(--ring), inset 0 0 0 var(--border-width) var(--color-border-subtle);
  }

  .full {
    height: 100%;
  }

  .p-none { padding: var(--space-0); gap: var(--space-0); overflow: hidden; }
  .p-sm { padding: var(--space-3); }
  .p-md { padding: var(--space-4); }
  .p-lg { padding: var(--space-6); }

  .interactive:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong), var(--ring);
  }

  .filled.interactive:hover {
    background: var(--color-fill);
    box-shadow: none;
  }

  .interactive:active {
    transform: scale(0.995);
  }
</style>
