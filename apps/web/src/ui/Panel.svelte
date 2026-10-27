<script>
  let { compact = false, children } = $props();
</script>

<section class:compact>{@render children()}</section>

<style>
  section { background: var(--panel); border-radius: var(--radius-2xl); padding: var(--space-7); margin-top: var(--space-6); }
  .compact { border-radius: var(--radius-xl); padding: var(--space-4); margin-top: 0; }
  @media (max-width: 767px) {
    section { border-radius: var(--radius-xl); padding: var(--space-5); margin-top: var(--space-5); }
  }
</style>
