<script>
  // KeysDialog connects a connector that has no sign-in, like Karbon or Shopify, with the keys it asks for: what
  // the person pastes from the service, each with where to find it. Secrets are typed as passwords, sent once
  // and never shown again.
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Button from './Button.svelte';
  import Text from './Text.svelte';
  import { connectKeys } from '../lib/app.svelte.js';

  let { open = $bindable(false), connector } = $props();

  let values = $state({});
  let busy = $state(false);
  const filled = $derived((connector.keys ?? []).every((k) => (values[k.name] ?? '').trim() !== ''));

  async function send(close) {
    busy = true;
    const ok = await connectKeys(connector.id, values);
    busy = false;
    if (ok) {
      values = {};
      close();
    }
  }
</script>

<Dialog bind:open title={`Connect ${connector.name}`}>
  {#each connector.keys ?? [] as k (k.name)}
    <Field label={k.label}>
      {#snippet children(id)}
        <Input {id} type={k.secret ? 'password' : 'text'} autocomplete="off" spellcheck="false" bind:value={values[k.name]} />
        {#if k.help}<Text size="sm" tone="secondary">{k.help}</Text>{/if}
      {/snippet}
    </Field>
  {/each}
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={!filled || busy} onclick={() => send(close)}>{busy ? 'Connecting…' : 'Connect'}</Button>
  {/snippet}
</Dialog>
