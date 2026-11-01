<script>
  let { d, mode = 'idle' } = $props();
</script>

<path class={['edge', mode]} {d} />

<style>
  .edge {
    fill: none;
    stroke: var(--color-border-strong);
    stroke-width: var(--border-width-thick);
    transition: stroke var(--duration-fast) var(--ease-out);
  }

  .active {
    stroke: var(--color-text-tertiary);
  }

  .dim {
    stroke: var(--color-border-subtle);
  }
</style>
