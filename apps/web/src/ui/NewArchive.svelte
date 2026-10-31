<script>
  import Icon from '#kit/Icon.svelte';
  import Bot from './Bot.svelte';
  import { fullName, TINTS, TYPES } from './looks.svelte.js';

  let { oncreate, onclose } = $props();
  const KINDS = [
    { key: 'business', hint: 'A company or team' },
    { key: 'family', hint: 'Shared with relatives' },
    { key: 'personal', hint: 'Just for you' },
  ];

  let name = $state('');
  let type = $state('family');
  let icon = $state('cottage');
  let tint = $state('#30a46c');
  let error = $state('');
  let busy = $state(false);
  let video = $state(false);

  const ready = $derived(name.trim() !== '' && !busy);
  const title = $derived(name.trim() ? fullName(name.trim(), type) : `Name your ${TYPES[type].label.toLowerCase()} archive`);

  function pick(key) {
    type = key;
    icon = TYPES[key].icons[1];
  }

  async function create(e) {
    e.preventDefault();
    if (!ready) return;
    busy = true;
    try {
      await oncreate(name.trim(), { type, icon, tint });
    } catch (err) {
      error = err.status === 403 ? 'Ask your admin to add an archive.' : err.message;
      busy = false;
    }
  }

  const esc = (e) => e.key === 'Escape' && onclose();
</script>

<svelte:window onkeydown={esc} />

<button class="scrim" type="button" aria-label="Close" onclick={onclose}></button>
<form class="sheet" role="dialog" aria-modal="true" aria-label="Create new archive" onsubmit={create}>
  <header>
    <h2>Create new archive</h2>
    <button class="close" type="button" aria-label="Close" onclick={onclose}><Icon name="x" /></button>
  </header>

  <div class="body">
    <div class="preview"><Bot seed="new" {icon} {tint} size={112} bare /></div>
    <p class="title" class:empty={!name.trim()}>{title}</p>
    <input class="name" bind:value={name} placeholder="Name your archive" aria-label="Archive name" {@attach (el) => el.focus()} />

    <div class="kinds" role="radiogroup" aria-label="Kind of archive">
      {#each KINDS as k (k.key)}
        <button type="button" role="radio" aria-checked={type === k.key} class:on={type === k.key} onclick={() => pick(k.key)}>
          <strong>{TYPES[k.key].label}</strong><small>{k.hint}</small>
        </button>
      {/each}
    </div>

    <div class="icons" role="radiogroup" aria-label="Character">
      {#each TYPES[type].icons as i (i)}
        <button type="button" role="radio" aria-checked={icon === i} aria-label={i} class:on={icon === i} onclick={() => (icon = i)}><Bot seed="new" icon={i} {tint} size={46} bare /></button>
      {/each}
    </div>

    <div class="tints" role="radiogroup" aria-label="Colour">
      {#each TINTS as t (t)}
        <button type="button" role="radio" aria-checked={tint === t} aria-label="Colour {t}" class:on={tint === t} style="background: {t}" onclick={() => (tint = t)}></button>
      {/each}
      <label class="any" title="Any colour"><input type="color" bind:value={tint} aria-label="Any colour" /></label>
    </div>

    <button class="help" type="button" aria-expanded={video} onclick={() => (video = !video)}>
      <span class="play"><Icon name="play" /></span>
      <span><strong>How archives work</strong><small>Watch the tour</small></span>
    </button>
    {#if video}
      <!-- svelte-ignore a11y_media_has_caption: the sound is only music, and the words are on screen. -->
      <video class="video" src="/tour.mp4" poster="/tour.jpg" controls autoplay playsinline {@attach (el) => el.scrollIntoView({ block: 'center', behavior: 'smooth' })}></video>
    {/if}
    {#if error}<p class="error">{error}</p>{/if}
  </div>

  <footer>
    <button class="cancel" type="button" onclick={onclose}>Cancel</button>
    <button class="create" type="submit" disabled={!ready}>{busy ? 'Creating' : 'Create'}</button>
  </footer>
</form>

<style>
  .scrim { position: fixed; inset: 0; z-index: calc(var(--z-nav) + 1); width: 100%; background: rgb(0 0 0 / 45%); cursor: default; }
  .sheet { position: fixed; top: 50%; left: 50%; z-index: calc(var(--z-nav) + 2); display: flex; flex-direction: column; width: min(460px, calc(100vw - 2 * var(--space-3))); max-height: calc(100svh - 2 * var(--space-3)); overflow: hidden; transform: translate(-50%, -50%); border-radius: var(--radius-panel); background: var(--surface-raised); box-shadow: 0 16px 48px rgb(0 0 0 / 30%); }
  header, footer { display: flex; flex: none; align-items: center; gap: var(--space-2); padding: var(--space-3) var(--space-4); }
  header { justify-content: space-between; border-bottom: var(--hairline) solid var(--fill); }
  footer { justify-content: flex-end; border-top: var(--hairline) solid var(--fill); }
  h2 { font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-sans); }
  .close { display: grid; place-items: center; width: 32px; height: 32px; color: var(--fg-muted); }
  .body { display: flex; flex-direction: column; align-items: center; gap: var(--space-3); padding: var(--space-4); overflow-y: auto; }
  .preview { display: grid; place-items: center; width: 136px; height: 124px; }
  .title { font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-serif); text-align: center; }
  .title.empty { color: var(--fg-muted); }
  .name { width: 100%; padding: var(--space-2) var(--space-3); border-radius: var(--radius-nav); background: var(--fill-faint); color: var(--fg); text-align: center; outline: 0; }
  .kinds { display: grid; grid-template-columns: repeat(3, 1fr); gap: var(--space-1); width: 100%; }
  .kinds button { display: flex; flex-direction: column; gap: 2px; padding: var(--space-2) var(--space-1); border-radius: var(--radius-nav); background: var(--fill-faint); line-height: var(--leading-snug); }
  .kinds .on { background: var(--primary); color: var(--on-primary); }
  .kinds strong { font-weight: var(--weight-medium); }
  .kinds small { color: inherit; font-size: var(--text-meta); opacity: 0.75; }
  .icons { display: flex; flex-wrap: wrap; justify-content: center; gap: var(--space-1); }
  .icons button { display: grid; place-items: center; width: 64px; height: 64px; border-radius: var(--radius-nav); }
  .icons .on { background: var(--fill); }
  .tints { display: flex; flex-wrap: wrap; justify-content: center; gap: var(--space-2); }
  .tints button { width: 26px; height: 26px; border-radius: 50%; box-shadow: inset 0 0 0 1px rgb(255 255 255 / 18%); }
  .tints .on { box-shadow: 0 0 0 2px var(--surface-raised), 0 0 0 4px var(--fg-muted); }
  .any { position: relative; width: 26px; height: 26px; overflow: hidden; border-radius: 50%; background: conic-gradient(#e5484d, #f5a524, #30a46c, #0091ff, #8e4ec6, #e5484d); }
  .any input { position: absolute; inset: 0; opacity: 0; cursor: pointer; }
  .help { display: flex; align-items: center; gap: var(--space-2); width: 100%; padding: var(--space-2); border-radius: var(--radius-nav); background: var(--fill-faint); text-align: left; }
  .help:hover { background: var(--fill); }
  .help span:nth-child(2) { display: flex; flex: 1; flex-direction: column; line-height: var(--leading-snug); }
  .help strong { font-weight: var(--weight-medium); }
  .help small { color: var(--fg-muted); font-size: var(--text-meta); }
  .play { display: grid; place-items: center; flex: none; width: 34px; height: 34px; border-radius: 50%; background: #ff3b30; color: #fff; }
  .play :global(.icon) { width: 15px; height: 15px; fill: currentColor; }
  .video { flex: none; width: 100%; aspect-ratio: 16 / 9; border-radius: 12px; }
  .error { color: var(--danger); font-size: var(--text-table); }
  .cancel, .create { padding: var(--space-1-5) var(--space-4); border-radius: var(--radius-full); }
  .cancel { background: var(--fill); }
  .create { background: var(--primary); color: var(--on-primary); }
  .create:disabled { opacity: 0.4; }
</style>
