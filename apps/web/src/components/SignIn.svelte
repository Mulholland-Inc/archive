<script>
  import { KeyRound } from '@lucide/svelte';
  import Art from './Art.svelte';
  import Heading from './Heading.svelte';
  import Button from './Button.svelte';
  import Input from './Input.svelte';
  import ConnectorIcon from './ConnectorIcon.svelte';
  import Text from './Text.svelte';
  import { API, api, devicePasskey, passkeyLogin, passkeys, rememberPasskey, said, signOut, Unauthorized } from '../lib/api.js';
  import { boot } from '../lib/app.svelte.js';
  import { fade } from '../lib/motion.js';

  let { title = 'Sign in to Archive', art = 'cli', next = '', done = boot } = $props();

  let error = $state('');
  let busy = $state(false);
  let providers = $state([]);
  let loaded = $state(false);
  let pending = $state(false);
  let code = $state('');
  let field = $state();
  let passkeyed = false;
  const first = passkeys && devicePasskey();

  $effect(() => field?.focus());

  api('/auth/methods')
    .then((m) => {
      providers = m.providers ?? [];
      pending = m.code;
    })
    .catch((e) => (error = e.message))
    .finally(() => (loaded = true));

  async function passkey() {
    error = '';
    busy = true;
    try {
      const out = await passkeyLogin();
      if (out.code) pending = passkeyed = true;
      else await done();
    } catch (err) {
      if (err.name !== 'NotAllowedError' && err.name !== 'AbortError') error = err.message;
    }
    busy = false;
  }

  async function verify(event) {
    event.preventDefault();
    error = '';
    busy = true;
    try {
      const out = await api('/auth/totp/verify', { method: 'POST', body: { code: code.replace(/\D/g, '') } });
      if (passkeyed) rememberPasskey(out.user.id, 'yes');
      await done();
    } catch (err) {
      pending = !(err instanceof Unauthorized);
      error = said(err);
      code = '';
    }
    busy = false;
  }

  async function back() {
    await signOut();
    pending = false;
    code = error = '';
  }
</script>

<div class="screen" transition:fade={{ speed: 'normal' }}>
  <div class="card">
    <Art name={art} size="lg" />
    <Heading level={1} size="xl" leading="inherit" wrap="normal">{pending ? 'Enter your code' : title}</Heading>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    {#if pending}
      <form class="methods" onsubmit={verify}>
        <Input bind:value={code} bind:element={field} aria-label="Code" inputmode="numeric" autocomplete="one-time-code" />
        <Button variant="primary" size="lg" type="submit" full loading={busy}>Verify</Button>
        <Button variant="ghost" size="lg" full onclick={back}>Back</Button>
      </form>
    {:else if loaded}
      <div class="methods">
        {#if passkeys && first}
          <Button variant="primary" size="lg" icon={KeyRound} full loading={busy} onclick={passkey}>Use a passkey</Button>
        {/if}
        {#each providers as p (p.id)}
          <a class="provider" href="{API}/auth/oidc/{encodeURIComponent(p.id)}{next ? `?next=${encodeURIComponent(next)}` : ''}">
            <ConnectorIcon id={p.id} size="xs" />
            <span>Continue with {p.name}</span>
          </a>
        {/each}
        {#if passkeys && !first}
          <Button variant={providers.length ? 'ghost' : 'primary'} size="lg" icon={KeyRound} full loading={busy} onclick={passkey}>Use a passkey</Button>
        {/if}
      </div>
      {#if !providers.length && !passkeys}
        <Text as="p" tone="secondary">This browser can’t use passkeys, and no sign-in provider is set up. Ask whoever runs this Archive to set one.</Text>
      {:else}
        <Text as="p" size="sm" tone="tertiary">Archive is invite-only. Sign in with the account you were invited under.</Text>
      {/if}
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
    max-width: var(--measure-signin);
    text-align: center;
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
