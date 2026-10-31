<script>
  let { size = 'md', active = false, label } = $props();
</script>

<svg class={['mark', size, { active }]} viewBox="0 0 24 24" role={label ? 'img' : undefined} aria-label={label} aria-hidden={label ? undefined : 'true'}>
  <rect class="lid" x="3" y="4" width="18" height="4.5" rx="1.25" />
  <rect class="box" x="4.5" y="10" width="15" height="10" rx="1.5" />
  <rect class="slot" x="9.5" y="12.75" width="5" height="1.6" rx="0.8" />
</svg>

<style>
  .mark {
    flex: none;
    width: var(--icon-lg);
    height: var(--icon-lg);
    overflow: visible;
  }

  .sm {
    width: var(--icon-md);
    height: var(--icon-md);
  }

  .lg {
    width: var(--icon-2xl);
    height: var(--icon-2xl);
  }

  .lid,
  .box {
    fill: currentColor;
  }

  .slot {
    fill: var(--color-canvas);
  }

  .lid {
    transform-origin: 12px 8px;
    transition: transform var(--duration-slow) var(--ease-emphasized);
  }

  .active .lid {
    animation: lift calc(var(--duration-slower) * 3) var(--ease-in-out) infinite;
  }

  @keyframes lift {
    0%,
    100% {
      transform: none;
    }
    50% {
      transform: translateY(-1.5px) rotate(-6deg);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .active .lid {
      animation: none;
    }
  }
</style>
