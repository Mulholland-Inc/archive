<script>
  let { children } = $props();
</script>

<div class="frame">{@render children()}</div>

<style>
  .frame {
    display: flex;
    height: var(--code-max);
    border-radius: var(--radius-lg);
    overflow: hidden;
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
  }

  .frame :global(.sidebar) {
    position: relative;
    transform: none;
    box-shadow: inset calc(var(--border-width) * -1) 0 0 var(--color-border-subtle);
  }

  .frame :global(.sidebar.collapsed) {
    width: var(--sidebar-rail);
  }
</style>
