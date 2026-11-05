<script>
  import { Check, X } from '@lucide/svelte';
  import Text from './Text.svelte';
  import Button from './Button.svelte';
  import IconButton from './IconButton.svelte';
  import Icon from './Icon.svelte';
  import FileInput from './FileInput.svelte';
  import SearchField from './SearchField.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import Spinner from './Spinner.svelte';
  import { run } from '../lib/api.js';
  import { ws } from '../lib/app.svelte.js';
  import { size, fileArt, toFile } from '../lib/live.js';

  const EXPORTS = `SELECT f.id, f.hash, f.name, f.mime, f.size, f.metadata, f.status, f.error, f.access, f.created_at, f.kind, f.parent, f.connection, f.edited_at FROM archive.file f
    WHERE f.kind = 'file' AND f.hash <> '' AND f.name ~* '\\.(zip|json|ged|gedcom|csv|txt|mbox|ics|vcf|db)$'
      AND NOT EXISTS (SELECT FROM archive.connection c WHERE c.id = f.connection AND c.provider = 'import')
      AND ($1::text = '' OR f.name ILIKE '%' || $1::text || '%')
    ORDER BY f.name ~* '\\.(zip|ged|gedcom|mbox|db)$' DESC, coalesce(f.edited_at, f.created_at) DESC, f.id LIMIT 50`;

  // The file a key connector reads: one from this device, dropped or chosen, or with `library`, one already in Archive, like a
  // Takeout zip a Google Drive connection copied in. `file` is the device's File, `held` the Archive file; one is set at a time.
  let { file = $bindable(), held = $bindable(), error = $bindable(''), library = false, busy = false, sent = 0, help = '' } = $props();

  let from = $state('device');
  let over = $state(false);
  let picker = $state();
  let q = $state('');
  let found = $state(null);
  let names = $state({});
  let asked = 0;

  $effect(() => {
    if (!library) return;
    run(`return await archive.query("SELECT DISTINCT c.id, c.provider FROM archive.connection c JOIN archive.file f ON f.connection = c.id");`).then((rows) => {
      const of = new Map((ws().catalog?.catalog.connectors ?? []).map((k) => [k.id, k.name]));
      names = Object.fromEntries(rows.map((x) => [x.id, of.get(x.provider) ?? x.provider]));
    });
  });

  $effect(() => {
    if (!library) return;
    const term = q.trim();
    const n = ++asked;
    const t = setTimeout(async () => {
      const list = await run(`return await archive.query(${JSON.stringify(EXPORTS)}, ${JSON.stringify(term)});`);
      if (n === asked) found = list.map((f) => toFile(f));
    }, term ? 200 : 0);
    return () => clearTimeout(t);
  });

  function pick(f) {
    if (!f) return;
    const tar = f.name.match(/\.(tgz|tar\.gz|tar)$/i)?.[0];
    error = tar ? `Archive reads .zip files, not ${tar}. Ask for the export again as .zip.` : '';
    file = error ? undefined : f;
  }

  function dropped(e) {
    e.preventDefault();
    over = false;
    pick(e.dataTransfer?.files?.[0]);
  }

  function leave(e) {
    if (!e.currentTarget.contains(e.relatedTarget)) over = false;
  }

  function switched(v) {
    error = '';
    if (v === 'device') held = undefined;
    else file = undefined;
  }

  const where = (f) => [f.connection ? (names[f.connection] ?? 'Synced') : 'Your files', f.size, f.modified].filter(Boolean).join(' · ');
</script>

<!-- Cut in the middle, as Finder does, so a Takeout part's -001.zip still shows -->
{#snippet named(name, small)}
  <span class={['name', { small }]} title={name}><span class="head">{name.slice(0, -10)}</span><span class="tail">{name.slice(-10)}</span></span>
{/snippet}

<div class="picker">
  {#if library}
    <SegmentedControl
      bind:value={from}
      full
      size="sm"
      label="Where the export is"
      options={[{ value: 'device', label: 'This device' }, { value: 'files', label: 'Your files' }]}
      onchange={switched}
    />
  {/if}

  {#if from === 'device'}
    <FileInput bind:element={picker} onchange={(e) => pick(e.currentTarget.files[0])} />
    {#if file}
      <div class="chosen">
        <img src={fileArt(file.name)} alt="" width="40" height="40" />
        <div class="what">
          {@render named(file.name)}
          {#if busy && sent}
            <div class="bar" role="progressbar" aria-label="Uploading" aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.floor(sent * 100)}>
              <span style:width="{sent * 100}%"></span>
            </div>
            <Text size="sm" tone="secondary" tabular>{size(file.size * sent)} of {size(file.size)}</Text>
          {:else}
            <Text size="sm" tone="secondary" tabular>{size(file.size)}{busy ? ' · Importing…' : ''}</Text>
          {/if}
        </div>
        {#if !busy}<IconButton icon={X} label="Remove" size="sm" onclick={() => (file = undefined)} />{/if}
      </div>
    {:else}
      <div
        class={['drop', { over }]}
        role="group"
        aria-label="Drop the export here"
        ondragover={(e) => e.dataTransfer?.types.includes('Files') && (e.preventDefault(), (over = true))}
        ondragleave={leave}
        ondrop={dropped}
      >
        <img class="art" src="/types/type-box.webp" alt="" width="56" height="56" />
        <div class="words">
          <Text weight="medium">{over ? 'Drop to add it' : 'Drop your export here'}</Text>
          {#if help}<Text size="sm" tone="secondary">{help}</Text>{/if}
        </div>
        <Button size="sm" data-autofocus onclick={() => picker.click()}>Choose File…</Button>
      </div>
    {/if}
  {:else}
    <div class="pane">
      <SearchField bind:value={q} size="sm" placeholder="Search your files" label="Search your files" />
      <div class="list" role="radiogroup" aria-label="Exports in your files">
        {#if found === null}
          <div class="empty"><Spinner size="sm" /></div>
        {:else if !found.length}
          <div class="empty">
            {#if q.trim()}
              <Text size="sm" tone="secondary">No export named “{q.trim()}”</Text>
            {:else}
              <img class="art" src="/types/type-folder.webp" alt="" width="48" height="48" />
              <Text weight="medium">No exports in your files yet</Text>
              <Text size="sm" tone="secondary">Google Takeout and Apple can save straight to Google Drive or Dropbox. Once Archive syncs it, it shows up here.</Text>
            {/if}
          </div>
        {:else}
          {#each found as f (f.id)}
            <button type="button" role="radio" aria-checked={held?.id === f.id} class={['row', { on: held?.id === f.id }]} disabled={busy} onclick={() => (held = held?.id === f.id ? undefined : f)}>
              <img src={fileArt(f.name, f.kind)} alt="" width="28" height="28" />
              <span class="what">
                {@render named(f.name, true)}
                <Text size="xs" tone="secondary" truncate>{where(f)}</Text>
              </span>
              <span class="tick">{#if held?.id === f.id}<Icon icon={Check} size="sm" />{/if}</span>
            </button>
          {/each}
        {/if}
      </div>
    </div>
    {#if held && !held.connection}<Text size="sm" tone="secondary">It moves under the import, so only you see it.</Text>{/if}
  {/if}
  {#if error}<Text size="sm" tone="danger" role="alert">{error}</Text>{/if}
</div>

<style>
  .picker {
    --pane: 17rem;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    min-width: 0;
  }

  .pane {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    height: var(--pane);
  }

  .drop {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    min-height: var(--pane);
    padding: var(--space-6) var(--space-5);
    border: 1.5px dashed var(--color-border-strong);
    border-radius: var(--radius-xl);
    background: var(--color-surface-sunken);
    text-align: center;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      border-color var(--duration-fast) var(--ease-out);
  }

  .drop.over {
    border-color: var(--color-accent);
    background: var(--color-accent-subtle);
  }

  .art {
    transition: transform var(--duration-normal) var(--ease-bounce);
  }

  .drop.over .art {
    transform: translateY(calc(var(--space-1) * -1)) scale(1.06);
  }

  .words {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    max-width: 22rem;
  }

  .chosen {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-3) var(--space-3) var(--space-4);
    border: var(--border-width) solid var(--color-border);
    border-radius: var(--radius-xl);
    background: var(--color-surface-sunken);
  }

  .what {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: var(--space-1);
    min-width: 0;
    text-align: left;
  }

  .name {
    display: flex;
    min-width: 0;
    font-weight: var(--weight-medium);
    white-space: pre;
  }

  .name.small {
    font-size: var(--text-sm);
  }

  .head {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .tail {
    flex: none;
  }

  .bar {
    height: 4px;
    overflow: hidden;
    border-radius: var(--radius-full);
    background: var(--color-fill);
  }

  .bar span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: var(--color-accent);
    transition: width var(--duration-normal) var(--ease-out);
  }

  .list {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
    overflow-y: auto;
    padding: var(--space-1);
    border: var(--border-width) solid var(--color-border);
    border-radius: var(--radius-xl);
    background: var(--color-surface-sunken);
  }

  .row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
    padding: var(--space-2) var(--space-3) var(--space-2) var(--space-2);
    border: 0;
    border-radius: var(--radius-lg);
    background: transparent;
    color: inherit;
    font: inherit;
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .row:hover:not(:disabled) {
    background: var(--color-fill-subtle);
  }

  .row.on {
    background: var(--color-accent-subtle);
  }

  .row:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: -2px;
  }

  .row img {
    flex: none;
  }

  .tick {
    display: inline-flex;
    flex: none;
    width: 1rem;
    color: var(--color-accent);
  }

  .empty {
    display: flex;
    flex: 1;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-6);
    text-align: center;
  }

  @media (prefers-reduced-motion: reduce) {
    .art,
    .bar span {
      transition: none;
    }
  }
</style>
