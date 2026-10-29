<script>
  import { reveal } from './reveal.js';

  let { title, body, band = false, center = false, wide = false, id, children } = $props();
</script>

<section class={['section', band && 'band']} {id}>
  {#if title || body}
    <div class={['wrap', 'head', center && 'center']}>
      {#if title}<h2 class="reveal" use:reveal>{#if typeof title === 'function'}{@render title()}{:else}{title}{/if}</h2>{/if}
      {#if body}<p class="lead reveal" use:reveal>{#if typeof body === 'function'}{@render body()}{:else}{body}{/if}</p>{/if}
    </div>
  {/if}
  {#if children}<div class={[!wide && 'wrap', 'section-body']}>{@render children()}</div>{/if}
</section>
