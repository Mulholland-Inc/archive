<script>
  import Icon from '#kit/Icon.svelte';
  import { look } from './looks.svelte.js';

  let { seed = '', size = 40, icon = '', bare = false } = $props();
  const l = $derived(look(seed));
  const shown = $derived(icon || l.icon);
  const drawn = $derived(['robot', 'safe', 'bookcase', 'cottage', 'townhouse', 'farmhouse', 'warehouse', 'factory', 'tower'].includes(shown));
  const robot = $derived(shown === 'robot');
  // On a tile the robot is its portrait; on its own it stands, and only its head turns.
  const stem = $derived(robot ? (bare ? 'turn-robot-head' : 'turn-face') : `turn-${shown}`);
  const src = (name) => `/bots/${name}.webp`;
</script>

<span class="bot" class:bare class:building={drawn && !robot} style="--tint: {l.tint}; --size: {size}px">
  {#if drawn}
    <span class="art">
      {#if robot && bare}<img src={src('turn-robot-body')} alt="" />{/if}
      {#each [0, 1, 2, 3] as i (i)}<img class="f f{i}" src={src(`${stem}-${i}`)} alt="" />{/each}
      <img class="lit" src={src(`${stem}-lit`)} alt="" />
    </span>
  {:else}
    <Icon name={shown} />
  {/if}
</span>

<style>
  .bot { position: relative; display: grid; place-items: center; flex: none; width: var(--size); height: var(--size); overflow: hidden; border-radius: 30%; background: var(--tint); color: #fff; transition: box-shadow var(--duration-normal) var(--ease-out), filter var(--duration-normal) var(--ease-out); }
  .bare { overflow: visible; background: none; }
  .bot :global(.icon) { width: 55%; height: 55%; }
  .art { position: absolute; inset: 0; }
  .bare .art { inset: -22%; }
  .building:not(.bare) .art { inset: -16%; }
  img { position: absolute; inset: 0; width: 100%; height: 100%; }

  /* The drawing turns to camera a frame at a time, and back again. */
  .f, .lit { opacity: 0; transition: opacity 1ms linear; }
  .f0 { opacity: 1; }
  .f1 { transition-delay: 150ms; }
  .f2 { transition-delay: 75ms; }
  .f3 { transition-delay: 0ms; }
  .bot:hover .f, :global(.row:hover) .f, :global(.avatar:hover) .f, :global(.now:hover) .f { opacity: 1; }
  .bot:hover .f1, :global(.row:hover) .f1, :global(.avatar:hover) .f1, :global(.now:hover) .f1 { transition-delay: 0ms; }
  .bot:hover .f2, :global(.row:hover) .f2, :global(.avatar:hover) .f2, :global(.now:hover) .f2 { transition-delay: 75ms; }
  .bot:hover .f3, :global(.row:hover) .f3, :global(.avatar:hover) .f3, :global(.now:hover) .f3 { transition-delay: 150ms; }
  .bot:hover .lit, :global(.row:hover) .lit, :global(.avatar:hover) .lit, :global(.now:hover) .lit { opacity: 1; transition-delay: 220ms; animation: glow 1.4s ease-in-out 400ms infinite; }

  .bot:not(.bare):hover, :global(.row:hover) .bot:not(.bare), :global(.avatar:hover) .bot:not(.bare), :global(.now:hover) .bot:not(.bare) { filter: brightness(1.08); box-shadow: 0 0 0 2px var(--tint), 0 0 14px var(--tint); }
  @keyframes glow {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }
  @media (prefers-reduced-motion: reduce) {
    .lit { animation: none !important; }
  }
</style>
