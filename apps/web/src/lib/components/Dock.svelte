<script lang="ts">
  import SegmentedControl from './SegmentedControl.svelte';
  import IconButton from './IconButton.svelte';
  let {
    route = $bindable(),
    routes,
    theme,
    floating = true,
    ontheme
  }: {
    route: string;
    routes: { value: string; label: string; icon?: string }[];
    theme: 'light' | 'dark';
    floating?: boolean;
    ontheme: () => void;
  } = $props();
</script>

<div class="dock" data-floating={floating || undefined}>
  <SegmentedControl bind:value={route} options={routes} variant="plain" label="Page" />
  <IconButton icon={theme === 'dark' ? 'sun' : 'moon'} label="Toggle theme" onclick={ontheme} />
</div>

<style>
  [data-floating] {
    position: fixed;
    right: var(--space-4);
    bottom: var(--space-4);
    z-index: var(--z-dock);
  }
  .dock {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-1);
    border-radius: var(--radius-lg);
    background: var(--popover);
    box-shadow: var(--shadow-popover);
    font-size: var(--text-sm);
    animation: dock-in var(--duration-slower) var(--ease-spring);
  }
  @media (max-width: 40rem) {
    [data-floating] {
      display: none;
    }
  }
  @keyframes dock-in {
    from {
      transform: translateY(var(--slide));
      opacity: 0;
    }
  }
</style>
