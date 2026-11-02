<script>
  let { value = 0, tone = 'accent', label = 'Progress', size = 'md' } = $props();
  const steps = 20;
  const step = $derived(Math.round(Math.max(0, Math.min(1, value)) * steps));
</script>

<span class={['bar', tone, size]} role="progressbar" aria-label={label} aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.round(value * 100)}>
  <span class="fill" style:--progress={step / steps}></span>
</span>

<style>
  .bar {
    display: block;
    width: 100%;
    height: var(--space-1);
    border-radius: var(--radius-full);
    background: var(--color-fill);
    overflow: hidden;
  }

  .sm {
    height: var(--space-0-5);
  }

  .fill {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: var(--color-accent);
    transform: scaleX(var(--progress));
    transform-origin: left;
    transition: transform var(--duration-slower) var(--ease-out);
  }

  .success .fill { background: var(--color-success); }
  .danger .fill { background: var(--color-danger); }
  .neutral .fill { background: var(--color-text-tertiary); }

</style>
