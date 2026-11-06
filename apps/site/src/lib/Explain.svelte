<script>
  import { onMount } from 'svelte';
  import { page as here } from '$app/state';
  import Button from '#kit/Button.svelte';
  import Icon from '#kit/Icon.svelte';
  import { EXPLAIN, FILM } from './explain.js';
  import { START } from './site.js';

  const REST = { closed: 3, read: 30 };
  const STARTS = FILM.map((_, i) => FILM.slice(0, i).reduce((n, c) => n + c.dur, 0));

  let dialog, video;
  let gone = $state(false);
  let now = $state(0);

  const at = $derived(STARTS.findLastIndex((s) => now >= s));
  const home = $derived(here.url.pathname === '/');

  function rest(kind) {
    gone = true;
    try { localStorage.explain = Date.now() + REST[kind] * 864e5; } catch {}
  }

  const play = () => video.play().catch(() => {});

  function seek(i) {
    video.currentTime = STARTS[i];
    play();
  }

  export function open() {
    rest('read');
    dialog.showModal();
    video.currentTime = 0;
    if (!matchMedia('(prefers-reduced-motion: reduce)').matches) play();
  }

  function key(e) {
    if (e.key === 'ArrowRight' && at < FILM.length - 1) seek(at + 1);
    if (e.key === 'ArrowLeft' && at > 0) seek(at - 1);
  }

  onMount(() => {
    if (new URLSearchParams(location.search).has('explain')) open();
  });
</script>

<div class={['topbar', home ? 'home' : gone && 'gone']}>
  <button type="button" class="topbar-open" onclick={open}>
    <span class="topbar-nudge">{EXPLAIN.nudge}</span>
    <strong>{EXPLAIN.title}</strong><span class="topbar-sub">, {EXPLAIN.subtitle}</span>
    <Icon name="arrow" />
  </button>
  {#if !home}<button type="button" class="icon-btn topbar-close" aria-label="Close" onclick={() => rest('closed')}><Icon name="close" /></button>{/if}
</div>

<dialog class="explain" bind:this={dialog} onkeydown={key} onclick={(e) => e.target === dialog && dialog.close()} onclose={() => video.pause()} aria-label={EXPLAIN.title}>
  <div class="explain-page">
    <button type="button" class="icon-btn explain-close" aria-label="Close" onclick={() => dialog.close()}><Icon name="close" /></button>
    <video bind:this={video} bind:currentTime={now} src="/art/explain.mp4" poster="/art/explain.webp" muted playsinline preload="none" aria-hidden="true" onclick={() => (video.paused ? play() : video.pause())}></video>
    <ol class="explain-script">
      {#each FILM as c}{#each c.lines as line}<li>{line}</li>{/each}{/each}
    </ol>
    <div class="explain-foot">
      <div class="explain-dots">
        {#each FILM as c, i}<button type="button" class={[i === at && 'on']} aria-label="Chapter {i + 1}" onclick={() => seek(i)}></button>{/each}
      </div>
      <Button href={START[0]} small>{START[1]}</Button>
    </div>
  </div>
</dialog>
