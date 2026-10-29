<script>
  import Art from './Art.svelte';
  import Button from './Button.svelte';
  import { APP } from './site.js';

  let { title, subtitle, lead, art, alt, home = false, band = false, plain = false, stack = false, actions, children } = $props();
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
  <section class="hero hero-top">
    <div class="wrap">
      <h1>{title}</h1>
      <div class="hero-grid">
        <div>
          <p class="subtitle">{subtitle}</p>
          <p class="lead">{lead}</p>
          {@render row()}
        </div>
        <div class="hero-art"><Art class="float" name={art} {alt} /></div>
      </div>
    </div>
  </section>
{:else}
  <section class={[!plain && 'hero', 'page-hero', band && 'band']}>
    <div class="wrap hero-grid">
      <div class={stack ? 'stack' : undefined}>
        <h1>{title}</h1>
        <p class="lead">{lead}</p>
        {@render row()}
        {@render children?.()}
      </div>
      <div class="hero-art"><Art class="float" name={art} {alt} /></div>
    </div>
  </section>
{/if}
