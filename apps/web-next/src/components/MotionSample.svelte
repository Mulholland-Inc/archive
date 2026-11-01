<script>
  let { duration, easing, label } = $props();
  let on = $state(false);
</script>

<button type="button" class="row" onclick={() => (on = !on)} aria-pressed={on}>
  <code>{label}</code>
  <span class="track"><span class={['ball', `d-${duration}`, `e-${easing}`, { on }]}></span></span>
</button>

<style>
  .row {
    display: grid;
    grid-template-columns: var(--col-md) 1fr;
    align-items: center;
    gap: var(--space-4);
    width: 100%;
    padding: var(--space-2);
    border-radius: var(--radius-md);
    text-align: start;
  }

  .row:hover {
    background: var(--color-fill-subtle);
  }

  code {
    color: var(--color-text-tertiary);
    font-family: var(--font-sans);
    font-size: var(--text-xs);
    letter-spacing: var(--tracking-normal);
  }

  .track {
    position: relative;
    height: var(--control-xs);
    border-radius: var(--radius-full);
    background: var(--color-fill);
  }

  .ball {
    position: absolute;
    inset-block: var(--space-0-5);
    inset-inline-start: var(--space-0-5);
    aspect-ratio: 1;
    border-radius: var(--radius-full);
    background: var(--color-accent);
    transition-property: inset-inline-start;
  }

  .on {
    inset-inline-start: calc(100% - var(--control-xs) + var(--space-0-5));
  }

  .d-fast { transition-duration: var(--duration-fast); }
  .d-normal { transition-duration: var(--duration-normal); }
  .d-slow { transition-duration: var(--duration-slow); }
  .d-slower { transition-duration: var(--duration-slower); }
  .e-out { transition-timing-function: var(--ease-out); }
  .e-in { transition-timing-function: var(--ease-in); }
  .e-in-out { transition-timing-function: var(--ease-in-out); }
  .e-emphasized { transition-timing-function: var(--ease-emphasized); }
</style>
