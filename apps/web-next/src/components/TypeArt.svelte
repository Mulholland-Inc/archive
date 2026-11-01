<script>
  let { icon, size = 'lg' } = $props();
  const src = $derived(`${import.meta.env.BASE_URL}types/type-${icon}.webp`);
</script>

<img class={['art', size]} {src} alt="" draggable="false" />

<style>
  .art {
    display: block;
    flex: none;
    object-fit: contain;
    user-select: none;
  }

  .lg {
    width: 72%;
    height: 72%;
  }

  .xs {
    width: var(--type-row-art);
    height: var(--type-row-art);
  }

  .sm {
    width: var(--type-art);
    height: var(--type-art);
    margin-block: calc(var(--space-4) * -1);
  }
</style>
