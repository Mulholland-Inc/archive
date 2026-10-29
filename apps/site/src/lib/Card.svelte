<script>
  import Art from './Art.svelte';
  import { reveal } from './reveal.js';

  let { href, title, art, text } = $props();
</script>

<a class="card reveal" {href} use:reveal
  ><div class="frame"><Art name={art} lazy /></div><h3>{title}</h3><p>{text}</p></a
>
