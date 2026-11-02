<script>
  let { size = 'md', label = 'Loading' } = $props();
</script>

<span class={['spinner', size]} role="status" aria-label={label}></span>

<style>
  .spinner {
    display: inline-block;
    flex: none;
    width: var(--icon-md);
    height: var(--icon-md);
    border-radius: var(--radius-full);
    border: var(--focus-width) solid var(--color-fill-strong);
    border-top-color: currentColor;
    animation: spin calc(var(--duration-slower) * 2) linear infinite;
  }

  .sm {
    width: var(--icon-sm);
    height: var(--icon-sm);
  }

  .lg {
    width: var(--icon-lg);
    height: var(--icon-lg);
  }

  .xl {
    width: var(--icon-2xl);
    height: var(--icon-2xl);
  }

  @keyframes spin {
    to {
      transform: rotate(1turn);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .spinner {
      animation-duration: 1.6s;
    }
  }
</style>
