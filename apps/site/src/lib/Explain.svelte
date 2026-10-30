<script>
  import { onMount } from 'svelte';
  import { page as here } from '$app/state';
  import Art from './Art.svelte';
  import Button from '#kit/Button.svelte';
  import Icon from '#kit/Icon.svelte';
  import { EXPLAIN, PAGES } from './explain.js';
  import { START } from './site.js';

  const REST = { closed: 3, read: 30 };

  let dialog;
  let gone = $state(false);
  let at = $state(0);

  const page = $derived(PAGES[at]);
  const home = $derived(here.url.pathname === '/');
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

  function rich(s) {
    const bits = s.split(/\[([^\]]+)\]\(([^)]+)\)/);
    return bits.flatMap((b, i) => (i % 3 === 0 ? [[b]] : i % 3 === 1 ? [[b, bits[i + 1]]] : []));
  }

  function key(e) {
    if (e.key === 'ArrowRight' && !last) at++;
    if (e.key === 'ArrowLeft' && at > 0) at--;
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

{#snippet text(s)}{#each rich(s) as [words, href]}{#if href}<a class="link" {href} onclick={() => dialog.close()}>{words}</a>{:else}{words}{/if}{/each}{/snippet}

<dialog class="explain" bind:this={dialog} onkeydown={key} onclick={(e) => e.target === dialog && dialog.close()} aria-label={EXPLAIN.title}>
  <div class="explain-page">
    <button type="button" class="icon-btn explain-close" aria-label="Close" onclick={() => dialog.close()}><Icon name="close" /></button>
    <div class="explain-art">
      {#key at}<Art name={page.art} alt={page.alt} />{/key}
    </div>
    <div class="explain-copy" aria-live="polite">
      {#each PAGES as p, i}
        <div class={['explain-text', i === at && 'on']}>
          <h2>{@render text(p.title)}</h2>
          {#if p.list}
            <dl class="explain-list">{#each p.list as [name, what]}<dt>{@render text(name)}</dt><dd>{what}</dd>{/each}</dl>
          {:else}
            <p>{@render text(p.text)}</p>
          {/if}
        </div>
      {/each}
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
