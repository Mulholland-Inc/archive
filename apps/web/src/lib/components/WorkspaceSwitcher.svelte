<script lang="ts">
  import Icon from './Icon.svelte';
  let { name, pressed = false, onclick }: { name: string; pressed?: boolean; onclick?: () => void } = $props();
</script>

<button class="switcher" data-pressed={pressed || undefined} aria-expanded={pressed} aria-label={`Space: ${name}`} {onclick}>
  <span class="logo"><Icon name="brand" size="sm" /></span>
  <span class="name">{name}</span>
  <Icon name="chevron-down" size="sm" />
</button>

<style>
  .switcher {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    max-width: 100%;
    height: var(--control-height);
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
    width: calc(var(--control-height) - var(--space-1));
    height: calc(var(--control-height) - var(--space-1));
    border-radius: var(--radius-sm);
    background: var(--brand);
    color: var(--brand-foreground);
  }
  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: var(--font-semibold);
  }
</style>
