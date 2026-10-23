<script lang="ts">
  import { AuthFrame, Button, Field, Input, Stack, Notice, Divider, Text } from '../lib';
  import { api, passkeySignIn, passkeysSupported, ApiError } from '../lib/api';
  import { load } from '../lib/session.svelte';
  import { query } from '../lib/router.svelte';
  import type { SigninMethods } from '../lib/types';

  let email = $state('');
  let sending = $state(false);
  let sent = $state(false);
  let passkeyBusy = $state(false);
  let error = $state(query().get('error') ?? '');
  let methods = $state<SigninMethods | null>(null);

  api.methods().then((m) => (methods = m)).catch(() => {});

  async function sendLink(e: SubmitEvent) {
    e.preventDefault();
    if (!email.trim()) {
      error = 'Enter the email address of your Archive account.';
      return;
    }
    sending = true;
    error = '';
    try {
      await api.magic(email.trim());
      sent = true;
    } catch (err) {
      error = (err as Error).message;
    } finally {
      sending = false;
    }
  }

  async function passkey() {
    passkeyBusy = true;
    error = '';
    try {
      await passkeySignIn();
      await load();
    } catch (err) {
      if ((err as DOMException).name === 'NotAllowedError') error = '';
      else error = err instanceof ApiError ? err.message : 'That passkey did not work. Try again or use an email link.';
    } finally {
      passkeyBusy = false;
    }
  }
</script>

<AuthFrame title={sent ? 'Check your email' : 'Sign in to Archive'} description={sent ? undefined : 'Your records, files and sources, in one place.'}>
  {#if sent}
    <Stack gap="4">
      <Text as="p">If {email} has an account, a sign-in link is on its way. It works for 15 minutes.</Text>
      {#if methods && !methods.mail}
        <Notice>This server has no mail set up, so the link is written to the server log instead.</Notice>
      {/if}
      <Button variant="ghost" icon="arrow-left" onclick={() => (sent = false)}>Use another address</Button>
    </Stack>
  {:else}
    <Stack gap="4">
      {#if error}<Notice tone="danger">{error}</Notice>{/if}
      <form onsubmit={sendLink} novalidate>
        <Stack gap="3">
          <Field label="Email" id="email">
            <Input id="email" type="email" bind:value={email} autocomplete="email" placeholder="you@example.com" autofocus />
          </Field>
          <Button type="submit" variant="primary" size="lg" block loading={sending}>Email me a sign-in link</Button>
        </Stack>
      </form>
      {#if passkeysSupported() || methods?.providers.length}
        <Divider label="or" />
        <Stack gap="2">
          {#if passkeysSupported()}
            <Button size="lg" block icon="key" loading={passkeyBusy} onclick={passkey}>Sign in with a passkey</Button>
          {/if}
          {#each methods?.providers ?? [] as p (p)}
            <Button size="lg" block icon="external" onclick={() => (location.href = `/auth/oidc/${encodeURIComponent(p)}`)}>Continue with {p}</Button>
          {/each}
        </Stack>
      {/if}
    </Stack>
  {/if}
</AuthFrame>
