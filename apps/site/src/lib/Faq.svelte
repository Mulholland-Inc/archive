<script>
  import Text from './Text.svelte';
  import { reveal } from './reveal.js';

  let { items, still = false } = $props();
</script>

<div class={['faq', !still && 'reveal']} use:reveal={!still}>
  {#each items as [q, a]}
    <details><summary>{q}</summary><p><Text text={a} /></p></details>
  {/each}
</div>
