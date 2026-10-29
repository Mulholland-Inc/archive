<script>
  let { plain = false, tight = false, children } = $props();
</script>

<div class="sheet" class:plain class:tight>
  <div class="body fade">{@render children()}</div>
</div>

<style>
  .sheet {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    border-radius: var(--radius-panel);
    background: var(--surface);
    overflow: clip;
  }
  .plain { background: none; }
  .body {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: var(--space-5);
    min-height: 0;
    padding: var(--gutter);
    overflow-y: var(--sheet-overflow);
  }
  .body > :global(*) { flex: none; }
  .tight .body { --table-bleed: var(--space-2); gap: var(--space-2); padding: var(--space-2); }
</style>
