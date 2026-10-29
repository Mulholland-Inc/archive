<script>
  import { reveal as watch } from './reveal.js';

  let { items, reveal = false } = $props();
</script>

<div class={['logos', reveal && 'reveal']} use:watch={reveal}>{#each items as item}<span>{item}</span>{/each}</div>
