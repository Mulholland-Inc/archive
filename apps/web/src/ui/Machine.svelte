<script>
  import { api } from '../api.js';
  import Icon from '#kit/Icon.svelte';
  import { look, TYPES } from './looks.svelte.js';

  let { name = '', seed = '' } = $props();
  let open = $state(false);
  let info = $state(null);
  let root = $state();

  api('/auth/workspace').then((w) => (info = w)).catch(() => {});

  const made = $derived(info ? new Date(info.created_at).toLocaleDateString(undefined, { day: 'numeric', month: 'long', year: 'numeric' }) : '');

  $effect(() => {
    if (!open) return;
    const away = (e) => !root.contains(e.target) && (open = false);
    const esc = (e) => e.key === 'Escape' && (open = false);
    addEventListener('pointerdown', away);
    addEventListener('keydown', esc);
    return () => {
      removeEventListener('pointerdown', away);
      removeEventListener('keydown', esc);
    };
  });
</script>

<div class="machine" bind:this={root}>
  {#if open}
    <div class="card" role="dialog" aria-label="About this archive">
      <div class="head"><Icon name="cloud" /><strong>{name}</strong><span class="demo">Demo</span></div>
      <dl>
        <dt>Kind</dt><dd>{TYPES[look(seed).type].label} archive</dd>
        <dt>Started</dt><dd>{made}</dd>
        <dt>Runs on</dt><dd>Google Cloud Run</dd>
        <dt>Region</dt><dd>us-central1, Iowa</dd>
        <dt>Database</dt><dd>Postgres 18, its own database</dd>
        <dt>Files</dt><dd>Google Cloud Storage</dd>
        <dt>Encryption</dt><dd>Its own data key</dd>
      </dl>
      <p>Your archive is a real database you own. It sleeps when nobody's using it and wakes when you ask something. You can take all of it with you at any time.</p>
    </div>
  {/if}
  <button type="button" aria-label="About this archive" aria-expanded={open} title="Where this archive runs" onclick={() => (open = !open)}><Icon name="cloud" /></button>
</div>

<style>
  .machine { position: fixed; right: var(--space-3); bottom: var(--space-3); z-index: var(--z-nav); }
  @media (max-width: 767px) { .machine { top: var(--space-3); right: var(--space-3); bottom: auto; } .card { top: calc(100% + var(--space-2)); bottom: auto; } }
  button { display: grid; place-items: center; width: 32px; height: 32px; border-radius: var(--radius-full); background: var(--surface); color: var(--fg-muted); box-shadow: 0 1px 3px rgb(0 0 0 / 15%); transition: color var(--duration-fast); }
  button:hover { color: var(--fg); }
  button :global(.icon) { width: var(--size-icon); height: var(--size-icon); }
  .card { position: absolute; right: 0; bottom: calc(100% + var(--space-2)); display: flex; flex-direction: column; gap: var(--space-3); width: min(300px, calc(100vw - 2 * var(--space-3))); padding: var(--space-4); border-radius: var(--radius-panel); background: var(--surface); box-shadow: 0 8px 28px rgb(0 0 0 / 22%); font-size: var(--text-table); }
  .head { display: flex; align-items: center; gap: var(--space-2); }
  .demo { margin-left: auto; padding: 2px var(--space-1-5); border-radius: var(--radius-full); background: var(--fill); color: var(--fg-muted); font-size: var(--text-meta); }
  dl { display: grid; grid-template-columns: max-content minmax(0, 1fr); gap: var(--space-1) var(--space-3); margin: 0; }
  dt { color: var(--fg-muted); }
  dd { margin: 0; }
  p { color: var(--fg-muted); line-height: var(--leading-snug); }
</style>
