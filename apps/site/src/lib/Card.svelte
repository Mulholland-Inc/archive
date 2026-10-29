<script>
  import Art from './Art.svelte';
  import { reveal } from './reveal.js';

  let { class: cls = '', href, n, title, art, text } = $props();
</script>

<a class={['card reveal', cls]} {href} use:reveal
  ><h3><span class="n">{n}.</span> {title}</h3><div class="frame"><Art name={art} /></div><p>{text}</p></a
>
