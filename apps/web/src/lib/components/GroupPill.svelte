<script lang="ts">
  import Icon from './Icon.svelte';
  let {
    status,
    glyph,
    label,
    collapsed = false,
    onclick
  }: { status: string; glyph?: string; label: string; collapsed?: boolean; onclick?: () => void } = $props();
</script>

<button class="pill" data-status={status} aria-expanded={!collapsed} {onclick}>
  <Icon name={glyph ?? `status-${status}`} size="lg" mono />
  <span>{label}</span>
</button>

<style>
  .pill {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    flex: none;
    height: var(--pill-height);
    padding: 0 var(--space-3-5) 0 var(--space-2);
    border-radius: var(--radius-full);
    background: var(--pill-background);
    color: var(--group-foreground);
    font-size: var(--text-base);
    white-space: nowrap;
    --glyph-mono-on: var(--pill-background);
    transition:
      transform var(--duration-normal) var(--ease-spring),
      filter var(--duration-fast) var(--ease-out);
  }
  .pill:hover {
    filter: var(--hover-filter);
  }
  .pill:active {
    transform: scale(var(--press-scale));
  }
  [data-status='done'] { --pill-background: var(--group-done); }
  [data-status='review'] { --pill-background: var(--group-review); }
  [data-status='progress'] { --pill-background: var(--group-progress); }
  [data-status='todo'] { --pill-background: var(--group-todo); }
  [data-status='backlog'] { --pill-background: var(--group-backlog); }
  [data-status='unscoped'] { --pill-background: var(--group-unscoped); }
  [data-status='canceled'],
  [data-status='duplicate'],
  [data-status='triage'] { --pill-background: var(--group-canceled); }
</style>
