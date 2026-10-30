<script>
  import { BRANDS } from '#kit/brands.js';
  import { reveal as watch } from './reveal.js';

  let { items, reveal = false, marquee = false } = $props();

  const rows = $derived(marquee ? [items.slice(0, Math.ceil(items.length / 2)), items.slice(Math.ceil(items.length / 2))] : []);
</script>

{#if marquee}
  <div class="marquee" role="list">
    {#each rows as row, r}
      <div class={['marquee-row', r && 'reverse']}>
        <div class="marquee-track">
          {#each [...row, ...row] as item, i}<span role={i < row.length ? 'listitem' : undefined} aria-hidden={i >= row.length || undefined}
              >{#if BRANDS[item]}<svg class="brand" viewBox={BRANDS[item][0]} aria-hidden="true">{@html BRANDS[item][1]}</svg>{/if}{item}</span
            >{/each}
        </div>
      </div>
    {/each}
  </div>
{:else}
  <div class={['logos', reveal && 'reveal']} use:watch={reveal}>{#each items as item}<span>{item}</span>{/each}</div>
{/if}
