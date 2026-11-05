<script>
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Button from './Button.svelte';
  import Text from './Text.svelte';
  import FileInput from './FileInput.svelte';
  import Disclosure from './Disclosure.svelte';
  import SearchField from './SearchField.svelte';
  import { ArrowUpRight } from '@lucide/svelte';
  import { connect, connectKeys, connectPlaces } from '../lib/app.svelte.js';
  import { uploadLarge } from '../lib/api.js';
  import { size } from '../lib/live.js';
  import { exportSources } from '../lib/exports.js';

  let { open = $bindable(false), connector, workspace } = $props();

  let values = $state({});
  let picked = $state({});
  let pickers = $state({});
  let busy = $state(false);
  let sent = $state(0);
  let error = $state('');
  let over = $state('');
  let sources = $state(false);
  const asks = $derived(connector.ask ? [connector.ask] : (connector.keys ?? []));
  const filled = $derived(asks.every((k) => (k.file ? picked[k.name] : (values[k.name] ?? '').trim() !== '')));

  const places = $derived(connector.ask?.places ? connector.ask : null);

  let query = $state('');
  let found = $state([]);
  let opening = $state(null);

  $effect(() => {
    if (!places || !open) return;
    const q = query;
    const t = setTimeout(async () => {
      const out = await connectPlaces(connector.id, q);
      if (q === query) found = out;
    }, q ? 120 : 0);
    return () => clearTimeout(t);
  });

  function pickPlace(place) {
    opening = place;
    busy = true;
    connect(connector.id, { [places.name]: place.value }, workspace);
  }

  function pick(k, file) {
    if (!file) return;
    const tar = file.name.match(/\.(tgz|tar\.gz|tar)$/i)?.[0];
    error = tar ? `Archive reads .zip files, not ${tar}. Ask for the export again as .zip.` : '';
    picked[k.name] = error ? undefined : file;
  }

  function dropped(k, e) {
    e.preventDefault();
    over = '';
    pick(k, e.dataTransfer?.files?.[0]);
  }

  async function send(close) {
    if (connector.ask) {
      busy = true;
      connect(connector.id, { [connector.ask.name]: values[connector.ask.name].trim() }, workspace);
      return;
    }
    busy = true;
    error = '';
    const keys = { ...values };
    try {
      for (const k of asks.filter((k) => k.file)) {
        sent = 0;
        keys[k.name] = await uploadLarge(picked[k.name], (p) => (sent = p));
      }
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
      close();
    } else if (ok) {
      error = ok.message ?? String(ok);
    }
  }
</script>

<Dialog bind:open title={asks.some((k) => k.file) ? `${connector.name} an export` : `Connect ${connector.name}`}>
  {#if places}
    <Field label={places.label}>
      {#snippet children(id)}
        <SearchField {id} bind:value={query} placeholder="Search by name" label={places.label} autocomplete="off" spellcheck="false" />
        {#if places.help}<Text size="sm" tone="secondary">{places.help}</Text>{/if}
      {/snippet}
    </Field>
    <ul class="places" aria-label="Health systems">
      {#each found as place (place.value + place.name)}
        <li>
          <button type="button" class="place" disabled={busy} onclick={() => pickPlace(place)}>
            <span class="name">{place.name}</span>
            {#if opening === place}<Text size="sm" tone="secondary">Opening…</Text>{:else if place.state}<Text size="sm" tone="secondary">{place.state}</Text>{/if}
          </button>
        </li>
      {:else}
        {#if query.trim()}<li><Text size="sm" tone="secondary">No health system by that name. Try the name on your patient portal’s sign-in page.</Text></li>{/if}
      {/each}
    </ul>
  {:else}
    {#each asks as k (k.name)}
      <Field label={k.label}>
        {#snippet children(id)}
          {#if k.file}
            <FileInput bind:element={pickers[k.name]} onchange={(e) => pick(k, e.currentTarget.files[0])} />
            <div
              class={['drop', { over: over === k.name }]}
              role="group"
              aria-label="Drop a file here"
              ondragover={(e) => e.dataTransfer?.types.includes('Files') && (e.preventDefault(), (over = k.name))}
              ondragleave={() => (over = '')}
              ondrop={(e) => dropped(k, e)}
            >
              <Text size="sm" tone={picked[k.name] ? 'default' : 'secondary'}>{picked[k.name] ? `${picked[k.name].name} · ${size(picked[k.name].size)}` : 'Drop the zip or file here'}</Text>
              <Button {id} size="sm" disabled={busy} onclick={() => pickers[k.name].click()}>{picked[k.name] ? 'Choose another…' : 'Choose a file…'}</Button>
            </div>
          {:else}
            <Input {id} type={k.secret ? 'password' : 'text'} autocomplete="off" spellcheck="false" bind:value={values[k.name]} />
          {/if}
          {#if k.help}<Text size="sm" tone="secondary">{k.help}</Text>{/if}
          {#if k.link}<Button size="sm" href={k.link} target="_blank" rel="noreferrer" iconEnd={ArrowUpRight}>Open {new URL(k.link).host}</Button>{/if}
        {/snippet}
      </Field>
      {#if k.file && connector.id === 'import'}
        <Disclosure bind:open={sources}>
          {#snippet summary()}<Text size="sm">Where do I get an export?</Text>{/snippet}
          <ul class="sources">
            {#each exportSources as x (x.name)}
              <li>
                <span><Text size="sm" weight="medium">{x.name}</Text> <Text size="sm" tone="secondary">{x.how}</Text></span>
                {#if x.url}<Button size="sm" variant="ghost" href={x.url} target="_blank" rel="noreferrer" iconEnd={ArrowUpRight}>Request</Button>{/if}
              </li>
            {/each}
          </ul>
        </Disclosure>
      {/if}
    {/each}
  {/if}
  {#if error}<Text size="sm" tone="danger" role="alert">{error}</Text>{/if}
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    {#if !places}
      <Button variant="primary" disabled={!filled || busy} onclick={() => send(close)}>{busy ? (sent ? `Uploading… ${Math.floor(sent * 100)}%` : 'Connecting…') : connector.ask ? `Continue to ${connector.by ?? connector.name}` : asks.some((k) => k.file) ? 'Import' : 'Connect'}</Button>
    {/if}
  {/snippet}
</Dialog>

<style>
  .drop {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-5) var(--space-4);
    border: var(--border-width) dashed var(--color-border-strong);
    border-radius: var(--radius-md);
    text-align: center;
    transition: background-color var(--duration-fast) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
  }

  .drop.over {
    border-color: var(--color-accent);
    background: var(--color-accent-subtle);
  }

  .sources {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin: 0;
    padding: var(--space-2) 0 0;
    list-style: none;
  }

  .sources li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
  }
  .places {
    display: flex;
    flex-direction: column;
    gap: var(--space-0-5);
    max-height: 18rem;
    margin: var(--space-2) 0 0;
    padding: 0;
    overflow-y: auto;
    list-style: none;
  }

  .place {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    width: 100%;
    min-height: var(--control-lg);
    padding: var(--space-1-5) var(--space-2);
    border: 0;
    border-radius: var(--radius-md);
    background: none;
    color: var(--color-text);
    font: inherit;
    font-size: var(--text-md);
    text-align: left;
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .place:hover:not(:disabled),
  .place:focus-visible {
    background: var(--color-fill-subtle);
  }

  .place:disabled {
    cursor: default;
  }

  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
