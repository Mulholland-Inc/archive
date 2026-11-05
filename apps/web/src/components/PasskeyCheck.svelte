<script>
  import { KeyRound } from '@lucide/svelte';
  import Art from './Art.svelte';
  import Button from './Button.svelte';
  import Dialog from './Dialog.svelte';
  import Heading from './Heading.svelte';
  import Text from './Text.svelte';
  import { passkeys, passkeyRegister, passkeyVerify } from '../lib/api.js';
  import { app, connectLink } from '../lib/app.svelte.js';

  let busy = $state(false);
  let error = $state('');

  async function check() {
    busy = true;
    error = '';
    try {
      const id = app.checking;
      if (app.passkey) await passkeyVerify();
      else if (!(await passkeyRegister(app.user.id)).verified) throw new Error('This passkey didn’t ask for your fingerprint, face or PIN. Try another.');
      app.passkey = app.verified = true;
      app.checking = null;
      connectLink(id);
    } catch (e) {
      if (e.name !== 'NotAllowedError' && e.name !== 'AbortError') error = e.message;
    }
    busy = false;
  }
</script>

<Dialog open label="Check it’s you" size="sm" onclose={() => (app.checking = null)}>
  <div class="check">
    <Art name="security" />
    <Heading level={2} size="lg" wrap="normal">Check it’s you</Heading>
    <Text as="p" tone="secondary">
      {#if !passkeys}This browser can’t use passkeys. Open Archive in one that can to connect a bank.
      {:else if app.passkey}Use your passkey before connecting a bank.
      {:else}Add a passkey before connecting a bank.{/if}
    </Text>
    {#if error}<Text as="p" size="sm" tone="danger" role="alert">{error}</Text>{/if}
    {#if passkeys}
      <Button variant="primary" size="lg" icon={KeyRound} full loading={busy} onclick={check} data-autofocus>{app.passkey ? 'Use a passkey' : 'Add a passkey'}</Button>
    {/if}
  </div>
</Dialog>

<style>
  .check {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-4);
    padding-block: var(--space-2);
    text-align: center;
  }
</style>
