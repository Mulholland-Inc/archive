<script>
  import Art from './Art.svelte';
  import { reveal } from './reveal.js';

  let { items } = $props();
</script>

<div class={['specs', items.length > 3 && 'four']}>
  {#each items as item}
    <svelte:element this={item.href ? 'a' : 'div'} class={['spec', item.href && 'card', 'reveal']} href={item.href} use:reveal>
      {#if item.art}<div class="spec-art"><Art name={item.art} alt={item.alt ?? ''} lazy /></div>{/if}
      <h3>{item.title}</h3>
      {#if item.text}<p>{item.text}</p>{/if}
      {#if item.items}<ul>{#each item.items as line}<li>{line}</li>{/each}</ul>{/if}
    </svelte:element>
  {/each}
</div>
