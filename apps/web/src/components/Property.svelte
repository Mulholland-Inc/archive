<script>
  import Icon from './Icon.svelte';

  let { label, icon, children } = $props();
</script>

<div class="property">
  <span class="label">{#if icon}<Icon {icon} size="sm" />{/if}{label}</span>
  <div class="value">{@render children()}</div>
</div>

<style>
  .property {
    display: grid;
    grid-template-columns: var(--col-md) minmax(0, 1fr);
    gap: var(--space-4);
    align-items: start;
    min-height: var(--control-md);
    padding-block: var(--space-1-5);
    font-size: var(--text-md);
  }

  .label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: var(--control-sm);
    color: var(--color-text-tertiary);
  }

  .value {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-1-5);
    min-width: 0;
    min-height: var(--control-sm);
    color: var(--color-text);
  }

  @media (max-width: 40rem) {
    .property {
      grid-template-columns: var(--col-sm) minmax(0, 1fr);
      gap: var(--space-3);
    }
  }
</style>
