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

  const admin = $derived(users?.find((u) => u.email === email)?.role === 'owner');
  const steps = $derived([
    { title: 'Add a file', href: '/files', done: files?.length > 0 },
    { title: 'Connect your apps and agents', connect: true, done: tokens?.length > 0 || connected.length > 0 },
    ...(admin ? [{ title: 'Invite someone', href: '/users', done: users.length > 1 }] : []),
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
        {s.title}
      </svelte:element>
    {/each}
    <button class="hide" type="button" aria-label="Hide these" title="Hide these" onclick={hide}><Icon name="x" /></button>
  </section>
{/if}

<style>
  section { position: relative; display: flex; flex: none; flex-wrap: wrap; align-items: center; justify-content: center; gap: var(--space-1) var(--space-6); width: 100%; max-width: var(--composer-width); margin-top: var(--space-2); }
  .step { color: var(--fg-muted); font-size: var(--text-meta); transition: color var(--duration-fast); }
  .step:hover { color: var(--fg); }
  .hide { position: absolute; top: 50%; right: 0; display: grid; place-items: center; width: 24px; height: 24px; transform: translateY(-50%); color: var(--fg-muted); opacity: 0; transition: opacity var(--duration-fast); }
  section:hover .hide, .hide:focus-visible { opacity: 0.7; }
  .hide:hover { opacity: 1; }
  .hide :global(.icon) { width: 13px; height: 13px; }
  @media (hover: none) { .hide { opacity: 0.7; } }
  @media (max-width: 767px) { section { gap: var(--space-1) var(--space-4); padding: 0 var(--space-5); } }
</style>
