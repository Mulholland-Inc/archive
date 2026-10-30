<script>
  import Icon from '#kit/Icon.svelte';
  import { look } from './looks.svelte.js';

  let { seed = '', size = 40, icon = '', bare = false } = $props();
  const l = $derived(look(seed));
  const shown = $derived(icon || l.icon);
  const drawn = $derived(['robot', 'safe', 'bookcase', 'cottage', 'townhouse', 'farmhouse', 'warehouse', 'factory', 'tower'].includes(shown));
  const base = (name) => `/bots/bot-${name}.webp`;
</script>

<span class="bot" class:bare style="--tint: {l.tint}; --size: {size}px">
  {#if shown === 'robot'}
    <span class="art">
      <img src={base('robot-body')} alt="" />
      <img class="head" src={base('robot-head')} alt="" />
      <img class="head lit" src={base('robot-head-lit')} alt="" />
    </span>
  {:else if drawn}
    <span class="art tilt">
      <img src={base(shown)} alt="" />
      <img class="lit" src={base(`${shown}-lit`)} alt="" />
    </span>
  {:else}
    <Icon name={shown} />
  {/if}
</span>

<style>
  .bot { position: relative; display: grid; place-items: center; flex: none; width: var(--size); height: var(--size); overflow: hidden; border-radius: 30%; background: var(--tint); color: #fff; }
  .bare { overflow: visible; background: none; }
  .bot :global(.icon) { width: 55%; height: 55%; }
  .art { position: absolute; inset: 0; transition: transform var(--duration-normal) var(--ease-out); }
  .bare .art { inset: -22%; }
  img { position: absolute; inset: 0; width: 100%; height: 100%; }
  .head { transform-origin: 50% 46%; transition: transform var(--duration-normal) var(--ease-out); }
  .lit { opacity: 0; transition: opacity var(--duration-normal) var(--ease-out); }
  .bot:hover .head, :global(.row:hover) .head, :global(.avatar:hover) .head { transform: rotate(-7deg) translateY(-1.5%); }
  .bot:hover .lit, :global(.row:hover) .lit, :global(.avatar:hover) .lit { opacity: 1; }
  .bot:hover .tilt, :global(.row:hover) .tilt, :global(.avatar:hover) .tilt { transform: rotate(-3deg) translateY(-2%); }
</style>
