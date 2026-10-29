<script>
  import { reveal as watch } from './reveal.js';

  let { name, alt = '', class: cls = '', reveal = false, lazy = false, width, height } = $props();

  let broken = $state(false);
  const src = $derived(`/art/${name}.webp`);
</script>

{#if broken}
  <div class={['art-missing', cls, reveal && 'reveal']} use:watch={reveal}>{src}</div>
{:else}
  <img
    class={[cls, reveal && 'reveal']}
    {src}
    {alt}
    {width}
    {height}
    loading={lazy ? 'lazy' : undefined}
    onerror={() => (broken = true)}
    use:watch={reveal}
  />
{/if}
