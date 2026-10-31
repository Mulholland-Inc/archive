<script>
  let { value = 0, tone = 'accent', label = 'Progress', size = 'md' } = $props();
  const steps = 20;
  const step = $derived(Math.round(Math.max(0, Math.min(1, value)) * steps));
</script>

<span class={['bar', tone, size]} role="progressbar" aria-label={label} aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.round(value * 100)}>
  <span class={['fill', `v${step}`]}></span>
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
    transform-origin: left;
    transition: transform var(--duration-slower) var(--ease-out);
  }

  .success .fill { background: var(--color-success); }
  .danger .fill { background: var(--color-danger); }
  .neutral .fill { background: var(--color-text-tertiary); }

  .v0 { transform: scaleX(0); }
  .v1 { transform: scaleX(0.05); }
  .v2 { transform: scaleX(0.1); }
  .v3 { transform: scaleX(0.15); }
  .v4 { transform: scaleX(0.2); }
  .v5 { transform: scaleX(0.25); }
  .v6 { transform: scaleX(0.3); }
  .v7 { transform: scaleX(0.35); }
  .v8 { transform: scaleX(0.4); }
  .v9 { transform: scaleX(0.45); }
  .v10 { transform: scaleX(0.5); }
  .v11 { transform: scaleX(0.55); }
  .v12 { transform: scaleX(0.6); }
  .v13 { transform: scaleX(0.65); }
  .v14 { transform: scaleX(0.7); }
  .v15 { transform: scaleX(0.75); }
  .v16 { transform: scaleX(0.8); }
  .v17 { transform: scaleX(0.85); }
  .v18 { transform: scaleX(0.9); }
  .v19 { transform: scaleX(0.95); }
  .v20 { transform: scaleX(1); }
</style>
