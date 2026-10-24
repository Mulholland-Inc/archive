<script lang="ts">
  import Icon from './Icon.svelte';
  import { brand } from '../brand.svelte';
  let { size }: { size?: 'sm' } = $props();
</script>

<!-- a module's logo is an SVG compiled into the server binary, never anything a person typed -->
{#if brand.logo}<span class="mark" data-size={size}>{@html brand.logo}</span>{:else}<Icon name="brand" {size} />{/if}

<style>
  .mark {
    display: grid;
    place-items: center;
    width: 100%;
    height: 100%;
    color: inherit;
  }
  .mark :global(svg) {
    width: 72%;
    height: 72%;
  }
</style>
