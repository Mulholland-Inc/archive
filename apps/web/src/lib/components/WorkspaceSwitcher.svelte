<script lang="ts">
  import Icon from './Icon.svelte';
  let { name, detail, pressed = false, onclick }: { name: string; detail?: string; pressed?: boolean; onclick?: () => void } = $props();
</script>

<button class="switcher" data-pressed={pressed || undefined} aria-expanded={pressed} {onclick}>
  <span class="logo"><Icon name="brand" /></span>
  <span class="names">
    <span class="name">{name}</span>
    {#if detail}<span class="detail">{detail}</span>{/if}
  </span>
  <Icon name="chevron-down" size="sm" />
</button>

<style>
  .switcher {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    max-width: 100%;
    height: calc(var(--control-height) + var(--space-2));
    padding: 0 var(--space-1-5) 0 var(--space-1);
    border-radius: var(--radius);
    text-align: start;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
  }
  .switcher:hover,
  [data-pressed] {
    background: var(--hover);
  }
  .switcher:active {
    transform: scale(var(--press-scale));
  }
  .logo {
    flex: none;
    display: grid;
    place-items: center;
    width: var(--control-height);
    height: var(--control-height);
    border-radius: var(--radius-md);
    background: var(--foreground);
    color: var(--highlight);
    --glyph-cut: var(--foreground);
  }
  .names {
    display: flex;
    flex-direction: column;
    min-width: 0;
    line-height: var(--leading-snug);
  }
  .name {
    font-weight: var(--font-semibold);
  }
  .detail {
    font-size: var(--text-sm);
    color: var(--muted-foreground);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
