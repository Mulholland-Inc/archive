<script>
  let { variant = 'text', lines = 1, size = 'md' } = $props();
</script>

{#if variant === 'text'}
  <span class="lines" aria-hidden="true">
    {#each { length: lines } as _, i (i)}
      <span class={['sk', 'text', size, { last: lines > 1 && i === lines - 1 }]}></span>
    {/each}
  </span>
{:else}
  <span class={['sk', variant, size]} aria-hidden="true"></span>
{/if}

<style>
  .lines {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    width: 100%;
  }

  .sk {
    display: block;
    border-radius: var(--radius-sm);
    background: linear-gradient(90deg, var(--color-fill) 0%, var(--color-fill-strong) 50%, var(--color-fill) 100%);
    background-size: 300% 100%;
    animation: shimmer calc(var(--duration-slower) * 4) var(--ease-in-out) infinite;
  }

  .text {
    height: var(--space-3);
    width: 100%;
  }

  .text.lg {
    height: var(--space-4);
  }

  .last {
    width: 60%;
  }

  .circle {
    width: var(--avatar-md);
    height: var(--avatar-md);
    border-radius: var(--radius-full);
  }

  .circle.lg {
    width: var(--avatar-lg);
    height: var(--avatar-lg);
  }

  .block {
    width: 100%;
    aspect-ratio: var(--thumb-ratio);
    border-radius: var(--radius-lg);
  }

  .row {
    width: 100%;
    height: var(--row-height);
    border-radius: var(--radius-md);
  }

  @keyframes shimmer {
    from {
      background-position: 100% 0;
    }
    to {
      background-position: -200% 0;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .sk {
      animation: none;
    }
  }
</style>
