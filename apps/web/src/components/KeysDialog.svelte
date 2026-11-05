<script>
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Button from './Button.svelte';
  import Text from './Text.svelte';
  import { ArrowUpRight } from '@lucide/svelte';
  import { connect, connectKeys } from '../lib/app.svelte.js';

  let { open = $bindable(false), connector } = $props();

  let values = $state({});
  let busy = $state(false);
  let error = $state('');
  const asks = $derived(connector.ask ? [connector.ask] : (connector.keys ?? []));
  const filled = $derived(asks.every((k) => (values[k.name] ?? '').trim() !== ''));

  async function send(close) {
    if (connector.ask) {
      busy = true;
      connect(connector.id, { [connector.ask.name]: values[connector.ask.name].trim() });
      return;
    }
    busy = true;
    error = '';
    const ok = await connectKeys(connector.id, values, undefined, true);
    busy = false;
    if (ok === true) {
      values = {};
      close();
    } else if (ok) {
      error = ok.message ?? String(ok);
    }
  }
</script>

<Dialog bind:open title={`Connect ${connector.name}`}>
  {#each asks as k (k.name)}
    <Field label={k.label}>
      {#snippet children(id)}
        <Input {id} type={k.secret ? 'password' : 'text'} autocomplete="off" spellcheck="false" bind:value={values[k.name]} />
        {#if k.help}<Text size="sm" tone="secondary">{k.help}</Text>{/if}
        {#if k.link}<Button size="sm" href={k.link} target="_blank" rel="noreferrer" iconEnd={ArrowUpRight}>Open {new URL(k.link).host}</Button>{/if}
      {/snippet}
    </Field>
  {/each}
  {#if error}<Text size="sm" tone="danger" role="alert">{error}</Text>{/if}
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={!filled || busy} onclick={() => send(close)}>{busy ? 'Connecting…' : connector.ask ? `Continue to ${connector.by ?? connector.name}` : 'Connect'}</Button>
  {/snippet}
</Dialog>
