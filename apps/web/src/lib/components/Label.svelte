<script lang="ts">
  let { name, color = 1 }: { name: string; color?: number } = $props();
</script>

<span class="label" data-color={color}><span class="dot"></span>{name}</span>

<style>
  .label {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    flex: none;
    height: var(--chip-height);
    padding: 0 var(--space-2) 0 var(--space-1-5);
    border-radius: var(--chip-radius);
    background: var(--chip-background);
    font-size: var(--text-sm);
    white-space: nowrap;
    user-select: none;
  }
  .dot {
    width: calc(var(--space-1-5) + var(--space-0-5));
    height: calc(var(--space-1-5) + var(--space-0-5));
    border-radius: var(--radius-full);
    background: var(--label-dot);
  }
  [data-color='1'] { --label-dot: var(--label-1); }
  [data-color='2'] { --label-dot: var(--label-2); }
  [data-color='3'] { --label-dot: var(--label-3); }
  [data-color='4'] { --label-dot: var(--label-4); }
  [data-color='5'] { --label-dot: var(--label-5); }
</style>
