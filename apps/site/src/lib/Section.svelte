<script>
  import { reveal } from './reveal.js';

  let { title, body, band = false, id, children } = $props();
</script>

{#snippet content()}
  {#if title}
    <div class="head">
      <h2 class="reveal" use:reveal>{#if typeof title === 'function'}{@render title()}{:else}{title}{/if}</h2>
      {#if body}<p class="body reveal" use:reveal>{#if typeof body === 'function'}{@render body()}{:else}{body}{/if}</p>{/if}
    </div>
  {/if}
  {@render children?.()}
{/snippet}

{#if band}
  <section class="band" {id}><div class="wrap"><div class="section">{@render content()}</div></div></section>
{:else}
  <section class="section" {id}><div class="wrap">{@render content()}</div></section>
{/if}
