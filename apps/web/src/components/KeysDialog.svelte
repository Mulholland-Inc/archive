<script>
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Button from './Button.svelte';
  import Text from './Text.svelte';
  import FileInput from './FileInput.svelte';
  import { connect, connectKeys } from '../lib/app.svelte.js';
  import { uploadLarge } from '../lib/api.js';

  let { open = $bindable(false), connector } = $props();

  let values = $state({});
  let picked = $state({});
  let pickers = $state({});
  let busy = $state(false);
  let sent = $state(0);
  let failed = $state('');
  const asks = $derived(connector.ask ? [connector.ask] : (connector.keys ?? []));
  const filled = $derived(asks.every((k) => (k.file ? picked[k.name] : (values[k.name] ?? '').trim() !== '')));

  async function send(close) {
    if (connector.ask) {
      busy = true;
      connect(connector.id, { [connector.ask.name]: values[connector.ask.name].trim() });
      return;
    }
    busy = true;
    failed = '';
    const keys = { ...values };
    try {
      for (const k of asks.filter((k) => k.file)) {
        sent = 0;
        keys[k.name] = await uploadLarge(picked[k.name], (p) => (sent = p));
      }
    } catch (e) {
      busy = false;
      sent = 0;
      failed = `Couldn’t upload it: ${e.message}`;
      return;
    }
    const ok = await connectKeys(connector.id, keys);
    busy = false;
    sent = 0;
    if (ok) {
      values = {};
      picked = {};
      close();
    }
  }
</script>

<Dialog bind:open title={asks.some((k) => k.file) ? `${connector.name} an export` : `Connect ${connector.name}`}>
  {#each asks as k (k.name)}
    <Field label={k.label}>
      {#snippet children(id)}
        {#if k.file}
          <FileInput bind:element={pickers[k.name]} onchange={(e) => (picked[k.name] = e.currentTarget.files[0])} />
          <Button {id} variant="secondary" disabled={busy} onclick={() => pickers[k.name].click()}>{picked[k.name] ? picked[k.name].name : 'Choose a file…'}</Button>
        {:else}
          <Input {id} type={k.secret ? 'password' : 'text'} autocomplete="off" spellcheck="false" bind:value={values[k.name]} />
        {/if}
        {#if k.help}<Text size="sm" tone="secondary">{k.help}</Text>{/if}
      {/snippet}
    </Field>
  {/each}
  {#if failed}<Text size="sm" tone="danger">{failed}</Text>{/if}
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={!filled || busy} onclick={() => send(close)}>{busy ? (sent ? `Uploading… ${Math.floor(sent * 100)}%` : 'Connecting…') : connector.ask ? `Continue to ${connector.by ?? connector.name}` : asks.some((k) => k.file) ? 'Import' : 'Connect'}</Button>
  {/snippet}
</Dialog>
