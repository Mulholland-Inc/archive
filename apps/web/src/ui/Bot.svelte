<script>
  import Icon from '#kit/Icon.svelte';
  import robot from '../../../site/static/art/robot.webp';
  import { look } from './looks.svelte.js';

  let { seed = '', size = 40 } = $props();
  const l = $derived(look(seed));
</script>

<span class="bot" style="--tint: {l.tint}; --size: {size}px">
  {#if l.icon === 'robot'}<img src={robot} alt="" />{:else}<Icon name={l.icon} />{/if}
</span>

<style>
  .bot { display: grid; place-items: center; flex: none; width: var(--size); height: var(--size); overflow: hidden; border-radius: 30%; background: var(--tint); color: #fff; }
  img { width: 62%; height: auto; }
  .bot :global(.icon) { width: 55%; height: 55%; }
</style>
