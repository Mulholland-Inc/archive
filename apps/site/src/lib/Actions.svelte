<script>
  import Button from '#kit/Button.svelte';
  import { START } from './site.js';

  let { links = [START, ['/docs', 'Read the docs']], children } = $props();
</script>

<div class="row">
  {#each links as [href, label], i}<Button {href} ghost={i > 0}>{label}</Button>{/each}
  {@render children?.()}
</div>
