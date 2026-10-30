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
  // Where the little indicator lights sit on each drawing, as a share of its height.
  const LEDS = { cottage: 72, townhouse: 56, farmhouse: 70, cabinet: 45, warehouse: 66, factory: 75, bank: 66, safe: 63, bookcase: 47 };
  const ledY = $derived(robot ? (bare ? 45 : float ? 76 : 73) : LEDS[shown]);
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
      <span class="leds" style="top: {ledY}%"><b></b><b></b><b></b></span>
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

  /* Eyes come on, and small hard indicator lights chatter like a droid's. No glow. */
  .lit { opacity: 0; }
  .bot:hover .lit, :global(.row:hover) .lit, :global(.avatar:hover) .lit, :global(.now:hover) .lit { opacity: 1; animation: eyes 2.4s step-end 260ms infinite; }
  .leds { position: absolute; left: 50%; display: flex; gap: 3%; width: 30%; justify-content: center; transform: translate(-50%, -50%); visibility: hidden; }
  .bot:hover .leds, :global(.row:hover) .leds, :global(.avatar:hover) .leds, :global(.now:hover) .leds { visibility: visible; }
  .leds b { width: 16%; aspect-ratio: 1; min-width: 2px; border-radius: 1px; background: rgb(0 0 0 / 28%); }
  .bot:hover .leds b, :global(.row:hover) .leds b, :global(.avatar:hover) .leds b, :global(.now:hover) .leds b { animation: 1.1s step-end 260ms infinite; }
  .bot:hover .leds b:nth-child(1), :global(.row:hover) .leds b:nth-child(1), :global(.avatar:hover) .leds b:nth-child(1), :global(.now:hover) .leds b:nth-child(1) { animation-name: red; }
  .bot:hover .leds b:nth-child(2), :global(.row:hover) .leds b:nth-child(2), :global(.avatar:hover) .leds b:nth-child(2), :global(.now:hover) .leds b:nth-child(2) { animation-name: blue; animation-duration: 0.7s; }
  .bot:hover .leds b:nth-child(3), :global(.row:hover) .leds b:nth-child(3), :global(.avatar:hover) .leds b:nth-child(3), :global(.now:hover) .leds b:nth-child(3) { animation-name: red; animation-duration: 1.7s; animation-delay: 500ms; }
  @keyframes eyes { 0%, 88% { opacity: 1; } 90%, 94% { opacity: 0; } 96%, 100% { opacity: 1; } }
  @keyframes red { 0%, 45% { background: #ff2d20; } 46%, 100% { background: rgb(0 0 0 / 28%); } }
  @keyframes blue { 0%, 30% { background: rgb(0 0 0 / 28%); } 31%, 70% { background: #1e8cff; } 71%, 100% { background: rgb(0 0 0 / 28%); } }
  @media (prefers-reduced-motion: reduce) {
    .f, .lit, .leds b { animation: none !important; }
  }
</style>
