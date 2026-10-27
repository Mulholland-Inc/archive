<script>
  let { children } = $props();
  let el = $state();
  let start = $state(true);
  let end = $state(true);

  function edges() {
    start = el.scrollLeft <= 0;
    end = el.scrollLeft + el.clientWidth >= el.scrollWidth - 1;
  }

  $effect(() => {
    edges();
    const o = new ResizeObserver(edges);
    o.observe(el);
    for (const c of el.children) o.observe(c);
    return () => o.disconnect();
  });
</script>

<div bind:this={el} class="bar" class:start class:end onscroll={edges}>{@render children()}</div>

<style>
  .bar {
    display: flex;
    flex: none;
    align-items: center;
    gap: var(--space-2);
    overflow-x: auto;
    mask-image: linear-gradient(to right, transparent, black var(--fade), black calc(100% - var(--fade)), transparent);
  }
  .start { mask-image: linear-gradient(to right, black calc(100% - var(--fade)), transparent); }
  .end { mask-image: linear-gradient(to right, transparent, black var(--fade)); }
  .start.end { mask-image: none; }
</style>
