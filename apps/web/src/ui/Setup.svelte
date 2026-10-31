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
    { title: 'Add a file', icon: 'file-up', href: '/files', done: files?.length > 0 },
    { title: 'Connect your apps and agents', icon: 'grid-2x2-plus', connect: true, done: tokens?.length > 0 || connected.length > 0 },
    ...(admin ? [{ title: 'Invite someone', icon: 'user-plus', href: '/users', done: users.length > 1 }] : []),
  ]);
  const ready = $derived(files && tokens && users);
  const left = $derived(steps.filter((s) => !s.done).length);

  function hide() {
    hideSetup();
    hidden = true;
  }
</script>

{#if ready && left && !hidden}
  <section aria-label="Get started">
    {#each steps.filter((s) => !s.done) as s (s.title)}
      <svelte:element this={s.connect ? 'button' : 'a'} class="step" type={s.connect ? 'button' : undefined} href={s.href} onclick={s.connect ? () => dispatchEvent(new CustomEvent('connect')) : undefined}>
        <Icon name={s.icon} />{s.title}
      </svelte:element>
    {/each}
    <button class="hide" type="button" aria-label="Hide these" title="Hide these" onclick={hide}><Icon name="x" /></button>
  </section>
{/if}

<style>
  section { display: flex; flex: none; flex-wrap: wrap; align-items: center; justify-content: center; gap: var(--space-1); width: 100%; max-width: var(--composer-width); }
  .step { display: flex; align-items: center; gap: var(--space-1); padding: var(--space-1) var(--space-2); border-radius: var(--radius-full); color: var(--fg-muted); font-size: var(--text-meta); transition: background var(--duration-fast), color var(--duration-fast); }
  .step:hover { background: var(--fill); color: var(--fg); }
  .step :global(.icon) { width: 15px; height: 15px; }
  .hide { display: grid; place-items: center; width: 24px; height: 24px; border-radius: 50%; color: var(--fg-muted); opacity: 0.6; transition: opacity var(--duration-fast); }
  .hide:hover { opacity: 1; }
  .hide :global(.icon) { width: 13px; height: 13px; }
</style>
