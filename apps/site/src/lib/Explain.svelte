<script>
  import { onMount } from 'svelte';
  import Art from './Art.svelte';
  import Button from '#kit/Button.svelte';
  import Icon from '#kit/Icon.svelte';
  import { EXPLAIN, PAGES } from './explain.js';
  import { START } from './site.js';

  // The bar stays away for a few days once closed, longer once the book is read. app.html hides it before
  // first paint while it rests, so it never jumps in.
  const REST = { closed: 3, read: 30 };

  let dialog;
  let gone = $state(false);
  let at = $state(0);

  const page = $derived(PAGES[at]);
  const last = $derived(at === PAGES.length - 1);

  function rest(kind) {
    gone = true;
    try { localStorage.explain = Date.now() + REST[kind] * 864e5; } catch {}
  }

  export function open() {
    at = 0;
    rest('read');
    dialog.showModal();
  }

  function key(e) {
    if (e.key === 'ArrowRight' && !last) at++;
    if (e.key === 'ArrowLeft' && at > 0) at--;
  }

  onMount(() => {
    if (new URLSearchParams(location.search).has('explain')) open();
  });
</script>

<div class={['topbar', gone && 'gone']}>
  <button type="button" class="topbar-open" onclick={open}>
    <span class="topbar-nudge">{EXPLAIN.nudge}</span>
    <strong>{EXPLAIN.title}</strong><span class="topbar-sub">, {EXPLAIN.subtitle}</span>
    <Icon name="arrow" />
  </button>
  <button type="button" class="icon-btn topbar-close" aria-label="Close" onclick={() => rest('closed')}><Icon name="close" /></button>
</div>

<dialog class="explain" bind:this={dialog} onkeydown={key} onclick={(e) => e.target === dialog && dialog.close()} aria-label={EXPLAIN.title}>
  <div class="explain-page">
    <button type="button" class="icon-btn explain-close" aria-label="Close" onclick={() => dialog.close()}><Icon name="close" /></button>
    <div class="explain-art">
      {#key at}<Art name={page.art} alt={page.alt} />{/key}
    </div>
    <div class="explain-copy" aria-live="polite">
      <small>{EXPLAIN.title}, {EXPLAIN.subtitle} · {at + 1} of {PAGES.length}</small>
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
