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
    min-width: var(--count-size);
    height: var(--count-size);
    padding: 0 var(--space-2);
    border-radius: var(--radius-full);
    background: var(--tint-2);
    color: var(--muted-foreground);
    font-size: var(--text-base);
    font-variant-numeric: tabular-nums;
    overflow: hidden;
    user-select: none;
  }
  .digits {
    grid-area: 1 / 1;
    animation: count-in var(--duration-slow) var(--ease-spring);
  }
  @keyframes count-in {
    from {
      transform: translateY(var(--count-offset));
      opacity: 0;
    }
  }
</style>
