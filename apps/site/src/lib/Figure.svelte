<script>
  import Art from './Art.svelte';

  let { art, alt, caption } = $props();
</script>

<figure><Art name={art} {alt} /><figcaption>{caption}</figcaption></figure>
