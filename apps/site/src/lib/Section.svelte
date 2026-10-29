<script>
  import { reveal } from './reveal.js';

  let { title, body, band = false, center = false, id, children } = $props();
</script>

<section class={['section', band && 'band']} {id}>
  {#if title}
    <div class={['wrap', 'head', center && 'center']}>
      <h2 class="reveal" use:reveal>{#if typeof title === 'function'}{@render title()}{:else}{title}{/if}</h2>
      {#if body}<p class="lead reveal" use:reveal>{#if typeof body === 'function'}{@render body()}{:else}{body}{/if}</p>{/if}
    </div>
  {/if}
  {#if children}<div class="wrap section-body">{@render children()}</div>{/if}
</section>
