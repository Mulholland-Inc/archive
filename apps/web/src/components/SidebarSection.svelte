<script>
  import { getContext } from 'svelte';

  let { title, children, action } = $props();
  const sidebar = getContext('sidebar');
  const collapsed = $derived(sidebar?.collapsed ?? false);
</script>

<section class={['section', { collapsed }]}>
  {#if title}
    <div class="title">
      <span>{title}</span>
      {#if action}{@render action()}{/if}
    </div>
  {/if}
  <div class="items">{@render children()}</div>
</section>

<style>
  .section {
    display: flex;
    flex-direction: column;
    gap: var(--space-0-5);
    min-width: 0;
  }

  .title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: var(--control-sm);
    padding: var(--space-1-5);
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    white-space: nowrap;
    transition: opacity var(--duration-normal) var(--ease-out);
  }

  .collapsed .title {
    opacity: 0;
  }

  .items {
    display: flex;
    flex-direction: column;
    gap: var(--space-px);
  }
</style>
