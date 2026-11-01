<script>
  let { name, size = 'md' } = $props();
  const src = $derived(`${import.meta.env.BASE_URL}art/${name}.webp`);
</script>

<img class={['art', size]} {src} alt="" draggable="false" />

<style>
  .art {
    display: block;
    flex: none;
    margin-inline: auto;
    object-fit: contain;
    user-select: none;
  }

  .md {
    height: var(--settings-art);
  }

  .banner {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .zoom {
    width: var(--art-zoom);
    max-width: none;
    height: auto;
  }

  .banner,
  .zoom {
    animation: art-in var(--duration-slower) var(--ease-out);
  }

  :global([data-theme='dark']) :is(.banner, .zoom) {
    filter: invert(0.9) hue-rotate(180deg);
  }

  @keyframes art-in {
    from {
      opacity: 0;
      transform: scale(1.03);
    }
  }
</style>
