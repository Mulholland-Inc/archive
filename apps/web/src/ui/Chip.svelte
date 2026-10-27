<script>
  let { name, pulse = false, ...rest } = $props();
</script>

<span class={['chip', pulse && 'pulse']} {...rest}>{name}</span>

<style>
  .chip {
    padding: var(--space-3);
    border-radius: var(--radius-pill);
    background: var(--raised);
    color: var(--text);
    font-size: var(--text-sm);
    font-weight: var(--weight-bold);
    line-height: 1;
    white-space: nowrap;
  }
  .pulse { animation: pulse 1s infinite alternate; }
  @keyframes pulse { to { opacity: 0.5; } }
</style>
