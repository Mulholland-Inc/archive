<script>
  let { compact = false, children } = $props();
</script>

<section class:compact>{@render children()}</section>

<style>
  section { display: flex; flex-direction: column; gap: var(--space-5); padding: var(--inset); background: var(--panel); border-radius: var(--radius-2xl); }
  .compact { gap: var(--space-2); padding: var(--space-3); border-radius: var(--radius-xl); }
  @media (max-width: 767px) {
    section { border-radius: var(--radius-xl); }
  }
</style>
