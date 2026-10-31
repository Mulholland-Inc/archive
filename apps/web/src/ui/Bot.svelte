<script>
  import Icon from '#kit/Icon.svelte';
  import { look, ui } from './looks.svelte.js';

  let { seed = '', size = 40, icon = '', tint = '', bare = false } = $props();
  const l = $derived(look(seed));
  const shown = $derived(icon || l.icon);
  const drawn = $derived(['robot', 'safe', 'bookcase', 'cottage', 'townhouse', 'farmhouse', 'cabinet', 'warehouse', 'factory', 'bank', 'archivist'].includes(shown));
  const robot = $derived(shown === 'robot');
  // On a tile the robot is its portrait; on its own it stands, and only its head turns.
  const float = $derived(ui.floating && !bare);
  const stem = $derived(robot ? (bare ? 'turn-robot-head' : float ? 'turn-float' : 'turn-face') : `turn-${shown}`);
  const src = (name) => `/bots/${name}.webp`;
  // With no tile behind it, the character itself carries the colour.
  const paint = $derived(float || bare);

  // Each character has its own way of moving: how far it leans and lifts, how quickly it turns, how often it looks around.
  const PERSONALITIES = [
    { tilt: '-3deg', lift: '-3%', pace: 70, rest: 2600 },
    { tilt: '2.5deg', lift: '-6%', pace: 55, rest: 1700 },
    { tilt: '-1.5deg', lift: '-2%', pace: 95, rest: 3400 },
    { tilt: '4deg', lift: '-4%', pace: 65, rest: 2100 },
  ];
  const me = $derived(PERSONALITIES[[...`${seed}${shown}`].reduce((n, c) => n + c.charCodeAt(0), 0) % PERSONALITIES.length]);

  let root = $state();
  let frame = $state(0);
  let awake = $state(false);
  let eyes = $state(false);
  let timer;
  const later = (fn, ms) => (timer = setTimeout(fn, ms));
  const still = () => matchMedia('(prefers-reduced-motion: reduce)').matches;

  function turn(to, pace, slowing, done) {
    if (frame === to) return done?.();
    frame += Math.sign(to - frame);
    later(() => turn(to, pace * slowing, slowing, done), pace);
  }

  function idle() {
    later(() => {
      const pick = Math.random();
      if (pick < 0.45) {
        eyes = false;
        later(() => ((eyes = true), idle()), 110);
      } else {
        eyes = false;
        turn(pick < 0.75 ? 2 : 1, me.pace * 1.6, 1, () => later(() => turn(3, me.pace * 1.6, 1, () => ((eyes = true), idle())), 420 + Math.random() * 500));
      }
    }, me.rest * (0.7 + Math.random() * 0.6));
  }

  function wake() {
    clearTimeout(timer);
    awake = true;
    if (still()) return ((frame = 3), (eyes = true));
    turn(3, me.pace, 1.15, () => ((eyes = true), idle()));
  }

  function sleep() {
    clearTimeout(timer);
    awake = false;
    eyes = false;
    if (still()) return (frame = 0);
    later(() => turn(0, me.pace * 1.5, 1.3), 80);
  }

  $effect(() => {
    const host = root.closest('.row, .avatar, .now, button') ?? root;
    host.addEventListener('pointerenter', wake);
    host.addEventListener('pointerleave', sleep);
    return () => {
      clearTimeout(timer);
      host.removeEventListener('pointerenter', wake);
      host.removeEventListener('pointerleave', sleep);
    };
  });
</script>

{#snippet layer(name, cls = '')}
  <span class="layer {cls}" style="--src: url({src(name)})">
    <img src={src(name)} alt="" />
    {#if paint}<i></i>{/if}
  </span>
{/snippet}

<span class="bot" class:bare class:float class:building={drawn && !robot} class:awake class:eyes class:facing={frame === 3 && awake} bind:this={root} style="--tint: {tint || l.tint}; --size: {size}px; --tilt: {me.tilt}; --lift: {me.lift}">
  {#if drawn}
    <span class="art">
      {#if robot && bare}{@render layer('turn-robot-body')}{/if}
      {#each [0, 1, 2, 3] as i (i)}{@render layer(`${stem}-${i}`, `f${frame === i ? ' show' : ''}`)}{/each}
      {#if shown !== 'archivist'}
        <img class="lit" src={src(`${stem}-lit`)} alt="" />
        <img class="panel" src={src(`${stem}-panel`)} alt="" />
        <i class="led red" style="--src: url({src(`${stem}-red`)})"></i>
        <i class="led blue" style="--src: url({src(`${stem}-blue`)})"></i>
      {/if}
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
  .art { position: absolute; inset: 0; transition: transform 620ms cubic-bezier(0.4, 0, 0.2, 1); }
  .awake .art { transform: translateY(var(--lift)) rotate(var(--tilt)); transition: transform 380ms cubic-bezier(0.34, 1.5, 0.64, 1); }
  .bare .art { inset: -22%; }
  .building:not(.bare) .art { inset: -16%; }
  img, .layer { position: absolute; inset: 0; width: 100%; height: 100%; }
  .layer { isolation: isolate; }
  /* A soft brushed sheen, so a colour reads as matte metal and not flat paint. */
  i { position: absolute; inset: 0; background: linear-gradient(135deg, color-mix(in srgb, var(--tint) 74%, white) 0%, var(--tint) 46%, color-mix(in srgb, var(--tint) 80%, black) 100%); mix-blend-mode: multiply; -webkit-mask: var(--src) center / 100% 100% no-repeat; mask: var(--src) center / 100% 100% no-repeat; }

  /* One frame shows at a time; the script steps through them both ways. */
  .f { opacity: 0; }
  .f.show { opacity: 1; }

  /* Eyes switch on, and small hard indicator lights chatter like a droid's. No glow. */
  .lit, .panel, .led { opacity: 0; }
  .eyes .lit { opacity: 1; }
  .facing .panel { opacity: 1; }
  .led { position: absolute; inset: 0; mix-blend-mode: normal; -webkit-mask: var(--src) center / 100% 100% no-repeat; mask: var(--src) center / 100% 100% no-repeat; }
  .red { background: #ff3b30; }
  .blue { background: #2f9bff; }
  .facing .red { animation: red 1.1s step-end infinite; }
  .facing .blue { animation: blue 0.7s step-end infinite; }
  @keyframes red { 0%, 45% { opacity: 1; } 46%, 100% { opacity: 0; } }
  @keyframes blue { 0%, 30% { opacity: 0; } 31%, 70% { opacity: 1; } 71%, 100% { opacity: 0; } }
  @media (prefers-reduced-motion: reduce) {
    .led { animation: none !important; }
    .art, .awake .art { transition: none; transform: none; }
  }
</style>
