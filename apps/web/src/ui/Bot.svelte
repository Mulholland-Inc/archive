<script>
  import Icon from '#kit/Icon.svelte';
  import { look, ui } from './looks.svelte.js';

  let { seed = '', size = 40, icon = '', bare = false } = $props();
  const l = $derived(look(seed));
  const shown = $derived(icon || l.icon);
  const drawn = $derived(['robot', 'safe', 'bookcase', 'cottage', 'townhouse', 'farmhouse', 'cabinet', 'warehouse', 'factory', 'bank'].includes(shown));
  const robot = $derived(shown === 'robot');
  // On a tile the robot is its portrait; on its own it stands, and only its head turns.
  const float = $derived(ui.floating && !bare);
  const stem = $derived(robot ? (bare ? 'turn-robot-head' : float ? 'turn-float' : 'turn-face') : `turn-${shown}`);
  const src = (name) => `/bots/${name}.webp`;
  // With no tile behind it, the character itself carries the colour.
  const paint = $derived(float || bare);
</script>

{#snippet layer(name, cls = '')}
  <span class="layer {cls}" style="--src: url({src(name)})">
    <img src={src(name)} alt="" />
    {#if paint}<i></i>{/if}
  </span>
{/snippet}

<span class="bot" class:bare class:float class:building={drawn && !robot} style="--tint: {l.tint}; --size: {size}px">
  {#if drawn}
    <span class="art">
      {#if robot && bare}{@render layer('turn-robot-body')}{/if}
      {#each [0, 1, 2, 3] as i (i)}{@render layer(`${stem}-${i}`, `f f${i}`)}{/each}
      <img class="lit" src={src(`${stem}-lit`)} alt="" />
    </span>
  {:else}
    <Icon name={shown} />
  {/if}
</span>

<style>
  .bot { position: relative; display: grid; place-items: center; flex: none; width: var(--size); height: var(--size); overflow: hidden; border-radius: 30%; background: var(--tint); color: #fff; transition: box-shadow var(--duration-normal) var(--ease-out), filter var(--duration-normal) var(--ease-out); }
  .bare, .float { overflow: visible; background: none; }
  .float.building .art { inset: -10% !important; }
  .bot :global(.icon) { width: 55%; height: 55%; }
  .art { position: absolute; inset: 0; }
  .bare .art { inset: -22%; }
  .building:not(.bare) .art { inset: -16%; }
  img, .layer { position: absolute; inset: 0; width: 100%; height: 100%; }
  .layer { isolation: isolate; }
  i { position: absolute; inset: 0; background: var(--tint); mix-blend-mode: multiply; -webkit-mask: var(--src) center / 100% 100% no-repeat; mask: var(--src) center / 100% 100% no-repeat; }

  /* The drawing turns to camera one frame at a time: only ever one frame shows. */
  .f { opacity: 0; }
  .f0 { opacity: 1; }
  .bot:hover .f0, :global(.row:hover) .f0, :global(.avatar:hover) .f0, :global(.now:hover) .f0 { animation: turn0 260ms step-end forwards; }
  .bot:hover .f1, :global(.row:hover) .f1, :global(.avatar:hover) .f1, :global(.now:hover) .f1 { animation: turn1 260ms step-end forwards; }
  .bot:hover .f2, :global(.row:hover) .f2, :global(.avatar:hover) .f2, :global(.now:hover) .f2 { animation: turn2 260ms step-end forwards; }
  .bot:hover .f3, :global(.row:hover) .f3, :global(.avatar:hover) .f3, :global(.now:hover) .f3 { animation: turn3 260ms step-end forwards; }
  @keyframes turn0 { 0% { opacity: 1; } 25%, 100% { opacity: 0; } }
  @keyframes turn1 { 0% { opacity: 0; } 25% { opacity: 1; } 50%, 100% { opacity: 0; } }
  @keyframes turn2 { 0%, 25% { opacity: 0; } 50% { opacity: 1; } 75%, 100% { opacity: 0; } }
  @keyframes turn3 { 0%, 50% { opacity: 0; } 75%, 100% { opacity: 1; } }

  .lit { opacity: 0; transition: opacity 120ms linear; }
  .bot:hover .lit, :global(.row:hover) .lit, :global(.avatar:hover) .lit, :global(.now:hover) .lit { opacity: 1; transition-delay: 240ms; animation: glow 1.4s ease-in-out 400ms infinite; }

  .bot:not(.bare):hover, :global(.row:hover) .bot:not(.bare), :global(.avatar:hover) .bot:not(.bare), :global(.now:hover) .bot:not(.bare) { filter: brightness(1.08); box-shadow: 0 0 0 2px var(--tint), 0 0 14px var(--tint); }
  .float:hover, :global(.row:hover) .float, :global(.avatar:hover) .float, :global(.now:hover) .float { box-shadow: none !important; filter: drop-shadow(0 0 7px var(--tint)) !important; }
  @keyframes glow {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }
  @media (prefers-reduced-motion: reduce) {
    .lit, .f { animation: none !important; }
  }
</style>
