<script>
  import { onMount } from 'svelte';
  import Art from './Art.svelte';
  import Button from '#kit/Button.svelte';
  import Icon from '#kit/Icon.svelte';
  import { EXPLAIN, PAGES } from './explain.js';
  import { START } from './site.js';

  // The nudge waits a little, then stays away for a few days once closed or read.
  const DELAY = 12_000;
  const REST = { closed: 3, read: 30 };
  const KEY = 'explain';

  let dialog;
  let shown = $state(false);
  let at = $state(0);

  const page = $derived(PAGES[at]);
  const last = $derived(at === PAGES.length - 1);

  function rest(kind) {
    try { localStorage[KEY] = Date.now() + REST[kind] * 864e5; } catch {}
  }

  function resting() {
    try { return Date.now() < +(localStorage[KEY] ?? 0); } catch { return false; }
  }

  export function open() {
    shown = false;
    at = 0;
    rest('read');
    dialog.showModal();
  }

  function close() {
    shown = false;
    rest('closed');
  }

  function key(e) {
    if (e.key === 'ArrowRight' && !last) at++;
    if (e.key === 'ArrowLeft' && at > 0) at--;
  }

  onMount(() => {
    if (new URLSearchParams(location.search).has('explain')) return open();
    if (resting()) return;
    const t = setTimeout(() => (shown = true), DELAY);
    return () => clearTimeout(t);
  });
</script>

<div class={['nudge', shown && 'shown']} inert={!shown}>
  <button type="button" class="nudge-open" onclick={open}>
    <span class="nudge-art"><Art name="hidden" /></span>
    <span><small>{EXPLAIN.nudge}</small><strong>{EXPLAIN.title}</strong></span>
    <Icon name="arrow" />
  </button>
  <button type="button" class="icon-btn" aria-label="Close" onclick={close}><Icon name="close" /></button>
</div>

<dialog class="explain" bind:this={dialog} onkeydown={key} onclick={(e) => e.target === dialog && dialog.close()} aria-label={EXPLAIN.title}>
  <div class="explain-page">
    <button type="button" class="icon-btn explain-close" aria-label="Close" onclick={() => dialog.close()}><Icon name="close" /></button>
    <div class="explain-art">
      {#key at}<Art name={page.art} alt={page.alt} />{/key}
    </div>
    <div class="explain-copy" aria-live="polite">
      <small>{at + 1} of {PAGES.length}</small>
      <h2>{page.title}</h2>
      {#if page.list}
        <ul class="explain-list">{#each page.list as [name, what]}<li><strong>{name}</strong> {what}</li>{/each}</ul>
      {:else}
        <p>{page.text}</p>
      {/if}
    </div>
    <div class="explain-foot">
      <div class="explain-dots">
        {#each PAGES as p, i}<button type="button" class={[i === at && 'on']} aria-label="Page {i + 1}" onclick={() => (at = i)}></button>{/each}
      </div>
      {#if at > 0}<Button ghost small onclick={() => at--}>Back</Button>{/if}
      {#if last}
        <Button href={START[0]} small>{START[1]}</Button>
      {:else}
        <Button small onclick={() => at++}>Next</Button>
      {/if}
    </div>
  </div>
</dialog>
