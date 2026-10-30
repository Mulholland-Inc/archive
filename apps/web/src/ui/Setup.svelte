<script>
  import { api, setupHidden, hideSetup } from '../api.js';
  import Icon from '#kit/Icon.svelte';

  let { email = '' } = $props();
  let files = $state(null);
  let tokens = $state(null);
  let users = $state(null);
  let hidden = $state(setupHidden());

  api('/api/files?limit=1').then((f) => (files = f)).catch(() => (files = []));
  api('/auth/tokens').then((t) => (tokens = t)).catch(() => (tokens = []));
  api('/auth/users').then((u) => (users = u)).catch(() => (users = []));

  const admin = $derived(!!users?.find((u) => u.email === email)?.admin);
  const steps = $derived([
    { title: 'Add a file', href: '/files', done: files?.length > 0 },
    { title: 'Connect your AI', href: 'https://archive.dev/docs', external: true, done: tokens?.length > 0 },
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
  <section>
    <div class="top">
      <h3>Get started</h3>
      <button type="button" onclick={hide}>Hide</button>
    </div>
    {#each steps as s (s.title)}
      <a class="step" class:done={s.done} href={s.href} target={s.external ? '_blank' : undefined}>
        <span class="mark">{#if s.done}<Icon name="check" />{/if}</span>
        <span>{s.title}</span>
      </a>
    {/each}
  </section>
{/if}

<style>
  section { display: flex; flex: none; flex-direction: column; gap: var(--space-1); width: 100%; max-width: var(--composer-width); padding: var(--space-3); border-radius: var(--radius-item); background: var(--surface); }
  .top { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-1); color: var(--fg-muted); font-size: var(--text-meta); }
  h3 { font-size: inherit; font-weight: var(--weight-medium); }
  button { color: inherit; transition: color var(--duration-fast); }
  button:hover { color: var(--fg); }
  .step { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-1-5); border-radius: var(--radius-nav); transition: background var(--duration-fast); }
  .step:hover { background: var(--fill-faint); }
  .done { color: var(--fg-muted); }
  .mark { display: grid; place-items: center; flex: none; width: var(--size-icon-lg); height: var(--size-icon-lg); border: var(--hairline) solid var(--fg-muted); border-radius: var(--radius-full); }
  .mark :global(.icon) { width: var(--size-icon); height: var(--size-icon); }
  .done .mark { border-color: transparent; background: var(--fill); }
</style>
