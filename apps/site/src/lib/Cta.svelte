<script>
  import Art from './Art.svelte';
  import Button from './Button.svelte';
  import { reveal } from './reveal.js';
  import { APP } from './site.js';

  let { title, lead, scene, alt, actions } = $props();
</script>

<section class="section cta-section">
  {#if scene}<div class="wrap scene reveal" use:reveal><Art name={scene} {alt} lazy /></div>{/if}
  <div class="wrap cta">
    <h2>{title}</h2>
    <p class="lead">{lead}</p>
    <div class="row">
      {#if actions}
        {@render actions()}
      {:else}
        <Button href={APP}>Start building free</Button>
        <Button href="/docs" ghost>Read the docs</Button>
      {/if}
    </div>
  </div>
</section>
