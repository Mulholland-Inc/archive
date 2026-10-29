<script>
  import Art from './Art.svelte';
  import { reveal } from './reveal.js';

  let { name, alt = '' } = $props();
</script>

<div class="scene reveal" use:reveal><Art {name} {alt} lazy /></div>
