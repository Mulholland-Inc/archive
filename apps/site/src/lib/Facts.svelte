<script>
  import Text from './Text.svelte';
  import { reveal } from './reveal.js';

  let { head, rows, wide = false } = $props();
</script>

{#snippet table()}
  <table class={['facts', wide && 'wide', !wide && 'reveal']} use:reveal={!wide}>
    <thead><tr>{#each head as h}<th>{h}</th>{/each}</tr></thead>
    <tbody>
      {#each rows as row}
        {#if typeof row === 'string'}
          <tr><th colspan={head.length}>{row}</th></tr>
        {:else}
          <tr>{#each row as cell}<td><Text text={cell} /></td>{/each}</tr>
        {/if}
      {/each}
    </tbody>
  </table>
{/snippet}

{#if wide}
  <div class="facts-wrap reveal" use:reveal>{@render table()}</div>
{:else}
  {@render table()}
{/if}
