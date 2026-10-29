<script>
  import Art from './Art.svelte';
  import { reveal } from './reveal.js';

  let { art, alt = '', flip = false, children } = $props();
</script>

<div class={['split', flip && 'flip']}>
  <div class="split-copy reveal" use:reveal>{@render children()}</div>
  <div class="split-art reveal" use:reveal><Art name={art} {alt} lazy /></div>
</div>
