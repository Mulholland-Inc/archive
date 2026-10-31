<script>
  import { onDestroy } from 'svelte';
  import { api, connectedApps, setupHidden, hideSetup } from '../api.js';
  import Icon from '#kit/Icon.svelte';

  let { email = '' } = $props();
  let files = $state(null);
  let tokens = $state(null);
  let users = $state(null);
  let hidden = $state(setupHidden());
  let connected = $state(connectedApps());
  const refresh = () => (connected = connectedApps());
  addEventListener('connected', refresh);
  onDestroy(() => removeEventListener('connected', refresh));

  api('/api/files?limit=1').then((f) => (files = f)).catch(() => (files = []));
  api('/auth/tokens').then((t) => (tokens = t)).catch(() => (tokens = []));
  api('/auth/users').then((u) => (users = u)).catch(() => (users = []));

  const admin = $derived(!!users?.find((u) => u.email === email)?.admin);
  const steps = $derived([
    { title: 'Add your first file', hint: 'Documents, photos or email', icon: 'file-up', href: '/files', done: files?.length > 0 },
    { title: 'Connect your apps and agents', hint: 'ChatGPT, Claude, Drive and more', icon: 'grid-2x2-plus', connect: true, done: tokens?.length > 0 || connected.length > 0 },
    ...(admin ? [{ title: 'Invite someone', hint: 'Share this archive', icon: 'user-plus', href: '/users', done: users.length > 1 }] : []),
  ]);
  const ready = $derived(files && tokens && users);
  const left = $derived(steps.filter((s) => !s.done).length);

  function hide() {
    hideSetup();
    hidden = true;
  }
</script>

{#if ready && left && !hidden}
  <section>
    <div class="top">
      <h3>Get started</h3>
      <button class="hide" type="button" onclick={hide}>Hide</button>
    </div>
    <div class="cards">
      {#each steps as s (s.title)}
        <svelte:element this={s.connect ? 'button' : 'a'} class="card" class:done={s.done} type={s.connect ? 'button' : undefined} href={s.href} onclick={s.connect ? () => dispatchEvent(new CustomEvent('connect')) : undefined}>
          <span class="glyph"><Icon name={s.done ? 'check' : s.icon} /></span>
          <span class="words"><strong>{s.title}</strong><small>{s.done ? 'Done' : s.hint}</small></span>
        </svelte:element>
      {/each}
    </div>
  </section>
{/if}

<style>
  section { display: flex; flex: none; flex-direction: column; gap: var(--space-2); width: 100%; max-width: var(--composer-width); }
  .top { display: flex; align-items: center; justify-content: space-between; padding: 0 var(--space-1); color: var(--fg-muted); font-size: var(--text-meta); }
  h3 { font-size: inherit; font-weight: var(--weight-medium); }
  .hide { color: inherit; transition: color var(--duration-fast); }
  .hide:hover { color: var(--fg); }
  .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: var(--space-2); }
  .card { display: flex; flex-direction: column; align-items: flex-start; gap: var(--space-2); width: 100%; padding: var(--space-3); border-radius: var(--radius-item); background: var(--surface); text-align: left; transition: background var(--duration-fast), transform var(--duration-fast) var(--ease-out); }
  .card:hover { background: var(--surface-raised); transform: translateY(-1px); }
  .glyph { display: grid; flex: none; place-items: center; width: 34px; height: 34px; border-radius: 30%; background: var(--fill); color: var(--fg); }
  .glyph :global(.icon) { width: 18px; height: 18px; }
  .words { display: flex; flex-direction: column; gap: 2px; line-height: var(--leading-snug); }
  strong { font-weight: var(--weight-medium); }
  small { color: var(--fg-muted); font-size: var(--text-meta); }
  .done { opacity: 0.6; }
  @media (max-width: 767px) {
    .cards { grid-template-columns: 1fr; }
    .card { flex-direction: row; align-items: center; }
  }
</style>
