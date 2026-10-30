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
      <img class="panel" src={src(`${stem}-panel`)} alt="" />
      <i class="led red" style="--src: url({src(`${stem}-red`)})"></i>
      <i class="led blue" style="--src: url({src(`${stem}-blue`)})"></i>
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
  .panel, .led { opacity: 0; }
  .led { position: absolute; inset: 0; mix-blend-mode: normal; -webkit-mask: var(--src) center / 100% 100% no-repeat; mask: var(--src) center / 100% 100% no-repeat; }
  .red { background: #ff3b30; }
  .blue { background: #2f9bff; }
  .bot:hover .panel, :global(.row:hover) .panel, :global(.avatar:hover) .panel, :global(.now:hover) .panel { opacity: 1; }
  .bot:hover .red, :global(.row:hover) .red, :global(.avatar:hover) .red, :global(.now:hover) .red { animation: red 1.1s step-end 260ms infinite; }
  .bot:hover .blue, :global(.row:hover) .blue, :global(.avatar:hover) .blue, :global(.now:hover) .blue { animation: blue 0.7s step-end 260ms infinite; }
  @keyframes eyes { 0%, 88% { opacity: 1; } 90%, 94% { opacity: 0; } 96%, 100% { opacity: 1; } }
  @keyframes red { 0%, 45% { opacity: 1; } 46%, 100% { opacity: 0; } }
  @keyframes blue { 0%, 30% { opacity: 0; } 31%, 70% { opacity: 1; } 71%, 100% { opacity: 0; } }
  @media (prefers-reduced-motion: reduce) {
    .f, .lit, .led { animation: none !important; }
  }
</style>
