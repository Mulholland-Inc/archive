<script lang="ts">
  import { Button, IconButton, Icon, Select } from '../../lib';
  import { session } from '../../lib/session.svelte';
  import { navigate } from '../../lib/router.svelte';
  import { fieldsOf, humanize, formatValue, kindOf, inputValue, parseInput } from '../../lib/spec';
  import { proposalTitle, proposalSource } from '../../lib/review';
  import type { Field, Proposal } from '../../lib/types';
  import Confidence from './Confidence.svelte';

  let {
    proposal,
    busy = false,
    onclose,
    onapprove,
    onreject
  }: {
    proposal: Proposal;
    busy?: boolean;
    onclose: () => void;
    onapprove: (edits: { [field: string]: unknown }) => void;
    onreject: () => void;
  } = $props();

  const o = $derived(session.ontology);
  const open = $derived(proposal.status === 'review' || proposal.status === 'accepted');
  const source = $derived(proposalSource(proposal));
  let text = $state<{ [field: string]: string }>({});

  const rows = $derived(
    fieldsOf(o, proposal.type).map(([n, f]) => ({ name: n, def: f, p: proposal.fields.find((x) => x.path === n) }))
  );

  function initial(def: Field, value: unknown) {
    return value === null || value === undefined ? '' : inputValue(o, def, value);
  }

  function editable(def: Field) {
    const k = kindOf(o, def.type);
    return !def.list && k !== 'reference' && k !== 'file' && k !== 'object' && k !== 'boolean';
  }

  function edits() {
    const out: { [field: string]: unknown } = {};
    for (const r of rows) {
      const t = text[r.name];
      if (t === undefined || t === initial(r.def, r.p?.value)) continue;
      out[r.name] = t.trim() === '' ? null : parseInput(o, r.def, t);
    }
    return out;
  }
</script>

<header class="bar">
  <IconButton icon="chevrons-right" label="Close" onclick={onclose} />
  <span class="crumb"><Icon name="inbox" size="sm" />Review<span class="sep">/</span>{humanize(proposal.type)}</span>
  <span class="grow"></span>
  {#if proposal.record_id}<Button variant="ghost" icon="maximize" onclick={() => navigate(`/records/${proposal.type}/${proposal.record_id}`)}>Open record</Button>{/if}
</header>

<div class="scroll">
  <h1 class="title">{proposalTitle(o, proposal)}</h1>
  <p class="meta">
    {#if proposal.file_id}<a class="u" href={`/library/${proposal.file_id}`}>{proposal.label}</a>{:else}{proposal.label || source.label}{/if}
    {#if !open}· {proposal.status === 'approved' ? 'Approved' : 'Rejected'}{proposal.decided_by ? ` by ${proposal.decided_by}` : ''}{/if}
  </p>

  <dl class="props">
    {#each rows as r (r.name)}
      {@const p = r.p}
      {@const k = kindOf(o, r.def.type)}
      {@const unsure = !!p && p.value !== null && p.value !== undefined && p.confidence < 0.9}
      <div class="prop">
        <dt><span>{humanize(r.name)}</span>{#if r.def.required}<i class="req" title="Required">*</i>{/if}</dt>
        <dd>
          <div class="value">
            {#if open && editable(r.def) && !(p?.secret && p.value == null && !r.def.required)}
              {#if k === 'enum' && r.def.enum}
                <Select
                  variant="field"
                  value={text[r.name] ?? initial(r.def, p?.value)}
                  options={[{ id: '', label: 'Empty' }, ...r.def.enum.map((v) => ({ id: v, label: humanize(v) }))]}
                  onchange={(v) => (text[r.name] = v)}
                  label={humanize(r.name)}
                />
              {:else}
                <input
                  class="edit"
                  value={text[r.name] ?? initial(r.def, p?.value)}
                  placeholder={p?.secret ? 'Hidden, type it to add it' : 'Empty'}
                  oninput={(e) => (text[r.name] = (e.target as HTMLInputElement).value)}
                  aria-label={humanize(r.name)}
                />
              {/if}
            {:else if p?.value !== null && p?.value !== undefined}
              <span class="text">{formatValue(o, r.def, p.value)}</span>
            {:else}
              <span class="empty">{p?.secret ? 'Hidden' : 'Not found'}</span>
            {/if}
            {#if unsure}<Confidence value={p!.confidence} />{/if}
          </div>
          {#if p?.route === 'drop' && p.value !== null && p.value !== undefined}
            <span class="note">Too unsure to use; type it above to keep it.</span>
          {:else if unsure && p?.original}
            <span class="note">“{p.original}”{#if p.page}{' '}<a class="u" href={`/library/${proposal.file_id}?page=${p.page}`}>page {p.page}</a>{/if}</span>
          {/if}
        </dd>
      </div>
    {/each}
  </dl>
</div>

{#if open}
  <footer class="foot">
    <span class="grow"></span>
    <Button variant="ghost" icon="x" disabled={busy} onclick={onreject}>Reject</Button>
    <Button variant="primary" icon="check" loading={busy} onclick={() => onapprove(edits())}>Approve</Button>
  </footer>
{/if}

<style>
  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-2) var(--space-3);
    border-bottom: var(--border-width) solid var(--tint-2);
  }
  .crumb,
  .meta {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-1-5);
    color: var(--muted-foreground);
    font-size: var(--text-sm);
  }
  .crumb {
    margin-left: var(--space-2);
  }
  .sep {
    color: var(--subtle-foreground);
  }
  .grow {
    flex: 1;
  }
  .scroll {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-6) var(--space-5) var(--space-8);
  }
  .title {
    font-family: var(--font-display);
    font-size: var(--text-2xl);
    font-weight: var(--font-bold);
    letter-spacing: -0.02em;
    line-height: 1.2;
    overflow-wrap: anywhere;
    margin-bottom: var(--space-2);
  }
  .meta {
    margin-bottom: var(--space-5);
  }
  .props {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  .prop {
    display: grid;
    grid-template-columns: 8rem 1fr;
    align-items: start;
    border-radius: var(--radius-sm);
  }
  dt {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-2);
    color: var(--muted-foreground);
    min-width: 0;
  }
  dt span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .req {
    font-style: normal;
    color: var(--red-500);
  }
  dd {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-1) var(--space-2);
    min-width: 0;
  }
  .value {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: 1.75rem;
  }
  .value :global(.anchor) {
    flex: 1;
  }
  .edit {
    flex: 1;
    min-width: 0;
    height: 1.75rem;
    padding: 0 var(--space-2);
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: inherit;
    font: inherit;
  }
  .edit:hover,
  .edit:focus {
    background: var(--card);
    box-shadow: inset 0 0 0 var(--border-width) var(--border);
    outline: none;
  }
  .text {
    flex: 1;
    padding: 0 var(--space-2);
    overflow-wrap: anywhere;
  }
  .empty {
    flex: 1;
    padding: 0 var(--space-2);
    color: var(--subtle-foreground);
  }
  .note {
    padding: 0 var(--space-2);
    color: var(--muted-foreground);
    font-size: var(--text-xs);
    overflow-wrap: anywhere;
  }
  .u {
    color: inherit;
    text-decoration: underline;
    text-decoration-color: var(--tint-4);
    text-underline-offset: 0.2em;
  }
  .foot {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    border-top: var(--border-width) solid var(--tint-2);
  }
</style>
