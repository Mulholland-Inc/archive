<script>
  import Art from './Art.svelte';
  import Button from './Button.svelte';
  import { APP } from './site.js';

  let { title, subtitle, lead, art, alt, home = false, actions, children } = $props();
</script>

{#snippet row()}
  <div class="row">
    {#if actions}
      {@render actions()}
    {:else}
      <Button href={APP}>Start building free</Button>
      <Button href="/docs" ghost>Read the docs</Button>
    {/if}
  </div>
{/snippet}

{#if home}
  <section class="home-hero">
    <div class="wrap home-copy">
      <h1>{title}</h1>
      <p class="subtitle">{subtitle}</p>
      <p class="lead">{lead}</p>
      {@render row()}
    </div>
    <div class="landscape"><Art name={art} {alt} /></div>
  </section>
{:else}
  <section class="page-hero">
    <div class="wrap">
      <div class="page-copy">
        <h1>{title}</h1>
        <p class="lead">{lead}</p>
        {@render row()}
        {@render children?.()}
      </div>
      <div class="page-art"><Art name={art} {alt} /></div>
    </div>
  </section>
{/if}
