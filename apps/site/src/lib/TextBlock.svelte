<script>
  import Art from './Art.svelte';
  import { reveal } from './reveal.js';

  let { class: cls = '', spec = false, art, title, items, children } = $props();
</script>

<div class={[spec ? 'spec' : 'c-text', cls, 'reveal']} use:reveal>
  {#if art}<Art name={art} />{/if}
  {#if title}<h3>{title}</h3>{/if}
  {#if items}<ul>{#each items as item}<li>{item}</li>{/each}</ul>{/if}
  {@render children?.()}
</div>
