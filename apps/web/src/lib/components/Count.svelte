<script lang="ts">
  let { value }: { value: number } = $props();
</script>

<span class="count">
  {#key value}<span class="digits">{value}</span>{/key}
</span>

<style>
  .count {
    display: inline-grid;
    place-items: center;
    flex: none;
    min-width: var(--chip-height);
    height: var(--chip-height);
    padding: 0 var(--space-1-5);
    border-radius: var(--radius-full);
    background: var(--tint-2);
    color: var(--muted-foreground);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    overflow: hidden;
    user-select: none;
  }
  .digits {
    grid-area: 1 / 1;
  }
</style>
