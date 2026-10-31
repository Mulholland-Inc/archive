<script>
  import Icon from '#kit/Icon.svelte';
  import Bot from './Bot.svelte';
  import { look, setLook, TINTS, TYPES } from './looks.svelte.js';

  let { name = '', seed = '', views = [], open = false, onclose } = $props();
  let picking = $state(false);

  const l = $derived(look(seed));

  function setType(type) {
    setLook(seed, { type, icon: TYPES[type].icons[1] });
  }
</script>

{#if open}<button class="veil" type="button" aria-label="Close" onclick={onclose}></button>{/if}
<aside class="panel" class:open aria-label={name}>
  <button class="avatar" type="button" aria-label="Change type, icon and colour" aria-expanded={picking} title="Change type, icon and colour" onclick={() => (picking = !picking)}>
    <Bot {seed} size={72} /><span class="edit"><Icon name="pencil" /></span>
  </button>
  <h2>{name}</h2>
  <p class="kind">{TYPES[l.type].label} archive</p>

  {#if picking}
    <div class="picker">
      <div class="types">
        {#each Object.entries(TYPES) as [key, t] (key)}
          <button type="button" class:on={l.type === key} onclick={() => setType(key)}>{t.label}</button>
        {/each}
      </div>
      <div class="icons">
        {#each TYPES[l.type].icons as icon (icon)}
          <button type="button" class:on={l.icon === icon} aria-label={icon} onclick={() => setLook(seed, { icon })}>
            <Bot {seed} {icon} size={36} bare />
          </button>
        {/each}
      </div>
      <div class="tints">
        {#each TINTS as tint (tint)}
          <button type="button" class:on={l.tint === tint} style="background: {tint}" aria-label="Colour {tint}" onclick={() => setLook(seed, { tint })}></button>
        {/each}
        <label class="any" title="Any colour"><input type="color" value={l.tint} aria-label="Any colour" oninput={(e) => setLook(seed, { tint: e.currentTarget.value })} /></label>
      </div>
    </div>
  {/if}

  <div class="list">
    {#if views.length}<h3>Lists</h3>{/if}
    {#each views as v (v.name)}
      <a class="link" href="/views/{v.name}"><Icon name={v.icon ?? 'list'} /><span>{v.name.replaceAll('_', ' ')}</span></a>
    {/each}
  </div>
</aside>

<style>
  .panel { display: flex; flex: none; flex-direction: column; align-items: center; gap: var(--space-3); width: 280px; max-height: 100svh; padding: var(--space-6) var(--space-4) calc(var(--space-6) + 40px); overflow-y: auto; }
  .avatar { position: relative; flex: none; }
  .edit { position: absolute; right: -6px; bottom: -6px; display: grid; place-items: center; width: 24px; height: 24px; border-radius: 50%; background: var(--surface); color: var(--fg-muted); box-shadow: 0 1px 3px rgb(0 0 0 / 20%); }
  .edit :global(.icon) { width: 13px; height: 13px; }
  h2 { font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-serif); text-align: center; }
  .kind { margin-top: calc(-1 * var(--space-2)); color: var(--fg-muted); font-size: var(--text-meta); text-align: center; }
  .picker { display: flex; flex: none; flex-direction: column; gap: var(--space-2); width: 100%; padding: var(--space-3); border-radius: var(--radius-item); background: var(--surface); }
  .types { display: flex; gap: var(--space-1); }
  .types button { flex: 1; padding: var(--space-1); border-radius: var(--radius-full); color: var(--fg-muted); font-size: var(--text-meta); }
  .types .on { background: var(--fill); color: var(--fg); }
  .icons { display: flex; flex-wrap: wrap; justify-content: center; gap: var(--space-1); }
  .icons button { display: grid; place-items: center; width: 52px; height: 52px; border-radius: var(--radius-nav); color: var(--fg-soft); }
  .icons .on { background: var(--fill-strong); color: var(--fg); }
  .tints { display: flex; justify-content: space-between; gap: 2px; }
  .tints button { width: 22px; height: 22px; border-radius: 50%; }
  .any { position: relative; width: 22px; height: 22px; overflow: hidden; border-radius: 50%; background: conic-gradient(#e5484d, #f5a524, #30a46c, #0091ff, #8e4ec6, #e5484d); }
  .any input { position: absolute; inset: 0; opacity: 0; cursor: pointer; }
  .tints .on { box-shadow: 0 0 0 2px var(--surface), 0 0 0 4px var(--fg-muted); }
  .list { display: flex; flex: none; flex-direction: column; gap: var(--space-0-5); width: 100%; font-size: var(--text-table); }
  .link { display: flex; align-items: center; gap: var(--space-2); width: 100%; padding: var(--space-1-5); border-radius: var(--radius-nav); text-align: left; text-transform: capitalize; transition: background var(--duration-fast); }
  .link:hover { background: var(--fill-faint); }
  h3 { padding: 0 var(--space-1-5); color: var(--fg-muted); font-size: var(--text-meta); font-weight: var(--weight-medium); }
  .veil { display: none; }
  @media (max-width: 1023px) {
    .panel { display: none; }
    .panel.open { position: fixed; top: 0; right: 0; bottom: 0; z-index: calc(var(--z-nav) + 2); display: flex; width: min(320px, calc(100vw - 56px)); max-height: none; border-radius: 20px 0 0 20px; background: var(--bg); box-shadow: -8px 0 32px rgb(0 0 0 / 16%); animation: slide var(--duration-normal) var(--ease-out); }
    .veil { position: fixed; inset: 0; z-index: calc(var(--z-nav) + 1); display: block; width: 100%; background: var(--scrim); }
  }
  @keyframes slide { from { transform: translateX(100%); } }
  @media (prefers-reduced-motion: reduce) { .panel.open { animation: none; } }
</style>
