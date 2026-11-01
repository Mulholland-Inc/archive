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
</style>
