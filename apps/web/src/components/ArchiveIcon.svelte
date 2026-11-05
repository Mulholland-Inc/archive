<script>
  import { untrack } from 'svelte';
  import { PERSONALITIES, personality, decoded, decode } from '../lib/looks.js';

  let { icon = 'cottage', tint = 0, seed = '', size = 'md', active = false } = $props();

  const mood = $derived(personality(`${seed}${icon}`));
  const me = $derived(PERSONALITIES[mood]);

  let root = $state();
  let frame = $state(0);
  let awake = $state(false);
  const url = $derived(`/bots/${icon}.webp`);
  let src = $state(untrack(() => (decoded(url) ? url : null)));
  let seen = $state(untrack(() => decoded(url)));
  let timer;
  const later = (fn, ms) => (timer = setTimeout(fn, ms));
  const still = () => matchMedia('(prefers-reduced-motion: reduce)').matches;

  function turn(to, pace, slowing, done) {
    if (frame === to) return done?.();
    frame += Math.sign(to - frame);
    later(() => turn(to, pace * slowing, slowing, done), pace);
  }

  function idle() {
    later(() => turn(Math.random() < 0.6 ? 2 : 1, me.pace * 1.6, 1, () => later(() => turn(3, me.pace * 1.6, 1, idle), 420 + Math.random() * 500)), me.rest * (0.7 + Math.random() * 0.6));
  }

  function wake() {
    clearTimeout(timer);
    awake = true;
    if (still()) return (frame = 3);
    turn(3, me.pace, 1.15, idle);
  }

  function sleep() {
    clearTimeout(timer);
    awake = false;
    if (still()) return (frame = 0);
    later(() => turn(0, me.pace * 1.5, 1.3), 80);
  }

  $effect(() => {
    if (!seen) return;
    const want = url;
    if (decoded(want)) src = want;
    else decode(want).then(() => want === url && (src = want));
  });

  $effect(() => {
    if (seen) return;
    const watch = new IntersectionObserver(([e]) => {
      if (!e.isIntersecting) return;
      seen = true;
      watch.disconnect();
    }, { rootMargin: '200px' });
    watch.observe(root);
    return () => watch.disconnect();
  });

  $effect(() => {
    if (!active) return;
    const t = setTimeout(wake);
    return () => clearTimeout(t);
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
  class={['bot', size, `mood-${mood}`, { awake }]}
  style:--tint={`var(--tint-${tint})`}
  style:--src={src ? `url(${src})` : null}
  aria-hidden="true"
>
  <span class="art">
    <span class="layer" style:--cell={frame}><i class="sheen"></i></span>
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

  .mood-0 { --tilt: var(--bot-tilt-0); --lift: var(--bot-lift-0); }
  .mood-1 { --tilt: var(--bot-tilt-1); --lift: var(--bot-lift-1); }
  .mood-2 { --tilt: var(--bot-tilt-2); --lift: var(--bot-lift-2); }
  .mood-3 { --tilt: var(--bot-tilt-3); --lift: var(--bot-lift-3); }

  .art {
    position: absolute;
    inset: 0;
    transition: transform var(--bot-turn) var(--ease-settle);
  }

  .awake .art {
    transform: translateY(var(--lift)) rotate(var(--tilt));
    transition: transform var(--bot-wake) var(--ease-bounce);
  }

  .layer {
    position: absolute;
    inset: 0;
    display: block;
    --at: calc(var(--cell) * 100% / 3) 0 / 400% 100% no-repeat;
    background: var(--src) var(--at);
    isolation: isolate;
  }

  .sheen {
    position: absolute;
    inset: 0;
    background: linear-gradient(var(--bot-sheen-angle), color-mix(in oklch, var(--tint) var(--bot-sheen-light), var(--tint-light)) 0%, var(--tint) var(--bot-sheen-mid), color-mix(in oklch, var(--tint) var(--bot-sheen-dark), var(--tint-dark)) 100%);
    mix-blend-mode: multiply;
    -webkit-mask: var(--src) var(--at);
    mask: var(--src) var(--at);
  }

  @media (prefers-reduced-motion: reduce) {
    .art,
    .awake .art {
      transition: none;
      transform: none;
    }
  }
</style>
