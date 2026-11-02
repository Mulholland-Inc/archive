<script>
  import { KeyRound } from '@lucide/svelte';
  import Art from './Art.svelte';
  import Button from './Button.svelte';
  import ConnectorIcon from './ConnectorIcon.svelte';
  import Text from './Text.svelte';
  import { API, api, devicePasskey, passkeyLogin, passkeys } from '../lib/api.js';
  import { boot } from '../lib/app.svelte.js';
  import { fade } from '../lib/motion.js';

  let error = $state('');
  let busy = $state(false);
  let providers = $state([]);
  let loaded = $state(false);
  const first = passkeys && devicePasskey();

  api('/auth/methods')
    .then((m) => (providers = m.providers ?? []))
    .catch((e) => (error = e.message))
    .finally(() => (loaded = true));

  async function passkey() {
    error = '';
    busy = true;
    try {
      await passkeyLogin();
      await boot();
    } catch (err) {
      if (err.name !== 'NotAllowedError' && err.name !== 'AbortError') error = err.message;
    }
    busy = false;
  }
</script>

<div class="screen" transition:fade={{ speed: 'normal' }}>
  <div class="card">
    <Art name="cli" size="lg" />
    <h1>Sign in to Archive</h1>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="methods">
      {#if passkeys && first}
        <Button variant="primary" size="lg" icon={KeyRound} full loading={busy} onclick={passkey}>Use a passkey</Button>
      {/if}
      {#each providers as p (p.id)}
        <a class="provider" href="{API}/auth/oidc/{encodeURIComponent(p.id)}">
          <ConnectorIcon id={p.id} size="xs" />
          <span>Continue with {p.name}</span>
        </a>
      {/each}
      {#if passkeys && !first}
        <Button variant={providers.length ? 'ghost' : 'primary'} size="lg" icon={KeyRound} full loading={busy} onclick={passkey}>Use a passkey</Button>
      {/if}
    </div>
    {#if loaded && !providers.length && !passkeys}
      <Text as="p" tone="secondary">This browser can’t use passkeys, and no sign-in provider is set up. Ask whoever runs this Archive to set one.</Text>
    {:else}
      <Text as="p" size="sm" tone="tertiary">Archive is invite-only. Sign in with the account you were invited under.</Text>
    {/if}
  </div>
</div>

<style>
  .screen {
    position: fixed;
    inset: 0;
    z-index: var(--z-dialog);
    display: grid;
    place-items: center;
    padding: var(--space-6);
    overflow: auto;
    background: var(--color-canvas);
  }

  .card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-5);
    width: 100%;
    max-width: 22rem;
    text-align: center;
  }

  h1 {
    font-family: var(--font-serif);
    font-size: var(--text-2xl);
    font-weight: var(--weight-serif);
    letter-spacing: var(--tracking-serif);
  }

  .methods {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    width: 100%;
  }

  .provider {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    height: var(--control-xl);
    padding: var(--space-3);
    border-radius: var(--radius-lg);
    background: var(--color-surface-raised);
    box-shadow: var(--ring);
    color: var(--color-text);
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .provider:hover {
    background: var(--color-fill);
  }

  .error {
    color: var(--color-danger);
    font-size: var(--text-sm);
  }
</style>
