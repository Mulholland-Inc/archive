<script>
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Button from './Button.svelte';
  import Text from './Text.svelte';
  import ExportPicker from './ExportPicker.svelte';
  import Disclosure from './Disclosure.svelte';
  import { ArrowUpRight } from '@lucide/svelte';
  import { connect, connectKeys } from '../lib/app.svelte.js';
  import { uploadLarge } from '../lib/api.js';
  import { exportSources } from '../lib/exports.js';

  let { open = $bindable(false), connector } = $props();

  let values = $state({});
  let picked = $state({});
  let held = $state({});
  let busy = $state(false);
  let sent = $state(0);
  let error = $state('');
  let sources = $state(false);
  const asks = $derived(connector.ask ? [connector.ask] : (connector.keys ?? []));
  const files = $derived(asks.some((k) => k.file));
  const filled = $derived(asks.every((k) => (k.file ? picked[k.name] || held[k.name] : (values[k.name] ?? '').trim() !== '')));

  async function send(close) {
    if (connector.ask) {
      busy = true;
      connect(connector.id, { [connector.ask.name]: values[connector.ask.name].trim() });
      return;
    }
    busy = true;
    error = '';
    const keys = { ...values };
    try {
      for (const k of asks.filter((k) => k.file)) {
        sent = 0;
        keys[k.name] = held[k.name]?.id ?? (await uploadLarge(picked[k.name], (p) => (sent = p)));
      }
      sent = 0;
    } catch (e) {
      busy = false;
      sent = 0;
      error = `Couldn’t upload it: ${e.message}`;
      return;
    }
    const ok = await connectKeys(connector.id, keys, undefined, true);
    busy = false;
    sent = 0;
    if (ok === true) {
      values = {};
      picked = {};
      held = {};
      close();
    } else if (ok) {
      error = ok.message ?? String(ok);
    }
  }
</script>

<Dialog bind:open title={files ? `${connector.name} an export` : `Connect ${connector.name}`} onclose={() => (error = '')}>
  {#if connector.id === 'import'}<Text size="sm" tone="secondary">Bring in the copy of your data a service gives you. Archive works out what it is and files it.</Text>{/if}
  {#each asks as k (k.name)}
    {#if k.file}
      <ExportPicker bind:file={picked[k.name]} bind:held={held[k.name]} bind:error library={connector.id === 'import'} {busy} {sent} help={k.help} />
    {:else}
      <Field label={k.label}>
        {#snippet children(id)}
          <Input {id} type={k.secret ? 'password' : 'text'} autocomplete="off" spellcheck="false" bind:value={values[k.name]} />
          {#if k.help}<Text size="sm" tone="secondary">{k.help}</Text>{/if}
          {#if k.link}<Button size="sm" href={k.link} target="_blank" rel="noreferrer" iconEnd={ArrowUpRight}>Open {new URL(k.link).host}</Button>{/if}
        {/snippet}
      </Field>
    {/if}
    {#if k.file && connector.id === 'import'}
      <Disclosure bind:open={sources}>
        {#snippet summary()}<Text size="sm">Where do I get an export?</Text>{/snippet}
        <ul class="sources">
          {#each exportSources as x (x.name)}
            <li>
              <span class="source"><Text size="sm" weight="medium">{x.name}</Text><Text size="xs" tone="secondary">{x.how}</Text></span>
              {#if x.url}<Button size="sm" variant="ghost" href={x.url} target="_blank" rel="noreferrer" iconEnd={ArrowUpRight}>Request</Button>{/if}
            </li>
          {/each}
        </ul>
      </Disclosure>
    {/if}
  {/each}
  {#if error && !files}<Text size="sm" tone="danger" role="alert">{error}</Text>{/if}
  {#snippet footer({ close })}
    <Button variant="ghost" disabled={busy} onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={!filled || busy} loading={busy} onclick={() => send(close)}>{busy ? (sent ? `Uploading ${Math.floor(sent * 100)}%` : files ? 'Importing…' : 'Connecting…') : connector.ask ? `Continue to ${connector.by ?? connector.name}` : files ? 'Import' : 'Connect'}</Button>
  {/snippet}
</Dialog>

<style>
  .sources {
    display: flex;
    flex-direction: column;
    margin: var(--space-2) 0 0;
    padding: 0;
    list-style: none;
  }

  .sources li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-2) 0;
  }

  .sources li + li {
    border-top: var(--border-width) solid var(--color-border-subtle);
  }

  .source {
    display: flex;
    flex-direction: column;
    gap: var(--space-0-5);
    min-width: 0;
  }
</style>
