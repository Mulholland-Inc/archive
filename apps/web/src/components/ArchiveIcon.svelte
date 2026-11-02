<script>
  import { PLAIN, PERSONALITIES, personality } from '../lib/looks.js';

  let { icon = 'robot', tint = 0, seed = '', size = 'md', label, active = false, greet = false } = $props();

  const stem = $derived(icon === 'robot' ? 'turn-float' : `turn-${icon}`);
  const lights = $derived(!PLAIN.includes(icon));
  const mood = $derived(personality(`${seed}${icon}`));
  const me = $derived(PERSONALITIES[mood]);

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
      eyes = false;
      if (pick < 0.45) later(() => ((eyes = true), idle()), 110);
      else turn(pick < 0.75 ? 2 : 1, me.pace * 1.6, 1, () => later(() => turn(3, me.pace * 1.6, 1, () => ((eyes = true), idle())), 420 + Math.random() * 500));
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
    if (!active && !greet) return;
    const t = setTimeout(wake, greet ? 300 : 0);
    const back = greet && !active ? setTimeout(sleep, 3200) : null;
    return () => {
      clearTimeout(t);
      clearTimeout(back);
    };
  });

  $effect(() => {
    if (active || matchMedia('(hover: none)').matches) return;
    const host = root.closest('button, a, [role="menuitem"], [role="menuitemcheckbox"]') ?? root;
    host.addEventListener('pointerenter', wake);
    host.addEventListener('pointerleave', sleep);
    return () => {
      clearTimeout(timer);
      host.removeEventListener('pointerenter', wake);
      host.removeEventListener('pointerleave', sleep);
    };
  });
</script>

<span
  bind:this={root}
  class={['bot', size, `tint-${tint}`, `mood-${mood}`, { building: icon !== 'robot', awake, eyes, facing: frame === 3 && awake }]}
  role={label ? 'img' : undefined}
  aria-label={label}
  aria-hidden={label ? undefined : 'true'}
>
  <span class="art">
    {#each [0, 1, 2, 3] as i (i)}
      <span class={['layer', 'f', `src-${stem}-${i}`, { show: frame === i }]}>
        <img src={`/bots/${stem}-${i}.webp`} alt="" />
        <i class="sheen"></i>
      </span>
    {/each}
    {#if lights}
      <img class="lit" src={`/bots/${stem}-lit.webp`} alt="" />
      <img class="panel" src={`/bots/${stem}-panel.webp`} alt="" />
      <i class={['led', 'red', `src-${stem}-red`]}></i>
      <i class={['led', 'blue', `src-${stem}-blue`]}></i>
    {/if}
  </span>
</span>

<style>
  .bot {
    position: relative;
    display: grid;
    place-items: center;
    flex: none;
    width: var(--avatar-md);
    height: var(--avatar-md);
    overflow: visible;
  }

  .xs { width: var(--avatar-xs); height: var(--avatar-xs); }
  .sm { width: var(--avatar-sm); height: var(--avatar-sm); }
  .lg { width: var(--avatar-lg); height: var(--avatar-lg); }
  .xl { width: var(--avatar-xl); height: var(--avatar-xl); }
  .xxl { width: var(--avatar-2xl); height: var(--avatar-2xl); }
  .rail { width: var(--archive-glyph); height: var(--archive-glyph); }

  .tint-0 { --tint: var(--tint-0); }
  .tint-1 { --tint: var(--tint-1); }
  .tint-2 { --tint: var(--tint-2); }
  .tint-3 { --tint: var(--tint-3); }
  .tint-4 { --tint: var(--tint-4); }
  .tint-5 { --tint: var(--tint-5); }
  .tint-6 { --tint: var(--tint-6); }
  .tint-7 { --tint: var(--tint-7); }

  .mood-0 { --tilt: -3deg; --lift: -3%; }
  .mood-1 { --tilt: 2.5deg; --lift: -6%; }
  .mood-2 { --tilt: -1.5deg; --lift: -2%; }
  .mood-3 { --tilt: 4deg; --lift: -4%; }

  .art {
    position: absolute;
    inset: 0;
    transition: transform var(--bot-turn) var(--ease-settle);
  }

  .building .art {
    inset: -10%;
  }

  .awake .art {
    transform: translateY(var(--lift)) rotate(var(--tilt));
    transition: transform var(--bot-wake) var(--ease-bounce);
  }

  img,
  .layer {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }

  .layer {
    isolation: isolate;
  }

  .sheen {
    position: absolute;
    inset: 0;
    background: linear-gradient(135deg, color-mix(in srgb, var(--tint) 74%, white) 0%, var(--tint) 46%, color-mix(in srgb, var(--tint) 80%, black) 100%);
    mix-blend-mode: multiply;
    -webkit-mask: var(--src) center / 100% 100% no-repeat;
    mask: var(--src) center / 100% 100% no-repeat;
  }

  .f {
    opacity: 0;
  }

  .f.show {
    opacity: 1;
  }

  .lit,
  .panel,
  .led {
    opacity: 0;
  }

  .eyes .lit,
  .facing .panel {
    opacity: 1;
  }

  .led {
    position: absolute;
    inset: 0;
    -webkit-mask: var(--src) center / 100% 100% no-repeat;
    mask: var(--src) center / 100% 100% no-repeat;
  }

  .red {
    background: var(--led-red);
  }

  .blue {
    background: var(--led-blue);
  }

  .facing .red {
    animation: red var(--duration-led-red) step-end infinite;
  }

  .facing .blue {
    animation: blue var(--duration-led-blue) step-end infinite;
  }

  @keyframes red {
    0%, 45% { opacity: 1; }
    46%, 100% { opacity: 0; }
  }

  @keyframes blue {
    0%, 30% { opacity: 0; }
    31%, 70% { opacity: 1; }
    71%, 100% { opacity: 0; }
  }

  @media (prefers-reduced-motion: reduce) {
    .led {
      animation: none;
    }

    .art,
    .awake .art {
      transition: none;
      transform: none;
    }
  }
</style>
