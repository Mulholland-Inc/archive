<script>
  import Icon from './Icon.svelte';

  let { icon, title, size = 'md', children } = $props();
</script>

<div class={['empty', size]}>
  {#if icon}
    <span class="glyph"><Icon {icon} size="lg" /></span>
  {/if}
  <p class="title">{title}</p>
  {#if children}<div class="actions">{@render children()}</div>{/if}
</div>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-4);
    padding: var(--space-16);
    text-align: center;
  }

  .sm {
    padding: var(--space-8);
    gap: var(--space-3);
  }

  .glyph {
    display: grid;
    place-items: center;
    width: var(--control-2xl);
    height: var(--control-2xl);
    border-radius: var(--radius-lg);
    background: var(--color-fill);
    color: var(--color-text-secondary);
  }

  .title {
    font-family: var(--font-serif);
    font-size: var(--text-xl);
    font-weight: var(--weight-serif);
    letter-spacing: var(--tracking-normal);
    color: var(--color-text);
  }

  .sm .title {
    font-family: var(--font-sans);
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
    letter-spacing: var(--tracking-snug);
    color: var(--color-text-secondary);
  }

  .actions {
    display: flex;
    gap: var(--space-2);
  }
</style>
