<script>
  let { token } = $props();
</script>

<div class="row">
  <code>--{token}</code>
  <span class={['bar', token]}></span>
</div>

<style>
  .row {
    display: grid;
    grid-template-columns: var(--col-sm) 1fr;
    align-items: center;
    gap: var(--space-4);
  }

  code {
    color: var(--color-text-tertiary);
    font-family: var(--font-sans);
    font-size: var(--text-xs);
    letter-spacing: var(--tracking-normal);
  }

  .bar {
    height: var(--space-3);
    border-radius: var(--radius-xs);
    background: var(--color-accent);
    opacity: 0.7;
  }

  .space-1 { width: var(--space-1); }
  .space-2 { width: var(--space-2); }
  .space-3 { width: var(--space-3); }
  .space-4 { width: var(--space-4); }
  .space-5 { width: var(--space-5); }
  .space-6 { width: var(--space-6); }
  .space-8 { width: var(--space-8); }
  .space-10 { width: var(--space-10); }
  .space-12 { width: var(--space-12); }
  .space-16 { width: var(--space-16); }
  .space-20 { width: var(--space-20); }
  .space-24 { width: var(--space-24); }
</style>
