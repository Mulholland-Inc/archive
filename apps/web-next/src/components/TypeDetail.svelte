<script>
  import { Check, Link, Workflow as WorkflowIcon } from '@lucide/svelte';
  import Tabs from './Tabs.svelte';
  import Stack from './Stack.svelte';
  import SettingsRow from './SettingsRow.svelte';
  import Table from './Table.svelte';
  import Badge from './Badge.svelte';
  import Chip from './Chip.svelte';
  import Icon from './Icon.svelte';
  import Text from './Text.svelte';
  import CodeBlock from './CodeBlock.svelte';
  import { ddl, typeLabels } from '../lib/schema.js';

  let { type, types = [], workflows = [], onopen, onworkflow } = $props();
  let tab = $state('overview');

  const byId = (id) => types.find((t) => t.id === id);
  const incoming = $derived(types.filter((t) => t.id !== type.id && t.links.some((l) => l.to === type.id)));
  const outgoing = $derived([...new Set(type.links.map((l) => l.to))].filter((id) => id !== type.id && byId(id)).map(byId));
  const self = $derived(type.links.some((l) => l.to === type.id));
  const flows = $derived(type.workflows.map((name) => workflows.find((w) => w.name === name) ?? { name }));
  const groups = $derived([
    { label: 'Owners', names: type.access.owners },
    { label: 'Editors', names: type.access.editors },
    { label: 'Viewers', names: type.access.viewers }
  ]);
  const shown = $derived(type.columns.filter((c) => c.name !== 'id' && c.name !== 'created_at').slice(0, 4));
  const sampleColumns = $derived(shown.map((c) => ({ key: c.name, label: c.name, muted: c !== shown[0] })));
  const items = $derived([
    { value: 'overview', label: 'Overview' },
    { value: 'columns', label: 'Columns', count: type.columns.length },
    { value: 'rows', label: 'Rows', count: type.rows.toLocaleString('en-US') },
    { value: 'sql', label: 'SQL' }
  ]);
  const columnColumns = [
    { key: 'name', label: 'Name' },
    { key: 'type', label: 'Type', width: 'sm' },
    { key: 'required', label: 'Required', width: 'sm' },
    { key: 'notes', label: 'Notes', muted: true }
  ];
  const columnRows = $derived(type.columns.map((c) => ({ ...c, id: c.name })));
</script>

<div class="detail">
  <Tabs bind:value={tab} {items} label="Type details" />
  <div class="pane">

    {#if tab === 'overview'}
      <Stack gap="0">
        <SettingsRow label="Rows"><Text tone="secondary" tabular>{type.rows.toLocaleString('en-US')}</Text></SettingsRow>
        <SettingsRow label="Columns"><Text tone="secondary" tabular>{type.columns.length}</Text></SettingsRow>
        <SettingsRow label="Last updated"><Text tone="secondary">{type.updated}</Text></SettingsRow>
        <SettingsRow label="Links to">
          <Stack direction="row" gap="1" wrap justify="end">
            {#each outgoing as t (t.id)}<Chip size="sm" icon={Link} onclick={() => onopen?.(t.id)}>{t.name}</Chip>{/each}
            {#if self}<Chip size="sm" icon={Link}>{type.name}</Chip>{/if}
            {#if !outgoing.length && !self}<Text tone="tertiary">None</Text>{/if}
          </Stack>
        </SettingsRow>
        <SettingsRow label="Linked from">
          <Stack direction="row" gap="1" wrap justify="end">
            {#each incoming as t (t.id)}<Chip size="sm" icon={Link} onclick={() => onopen?.(t.id)}>{t.name}</Chip>{/each}
            {#if !incoming.length}<Text tone="tertiary">None</Text>{/if}
          </Stack>
        </SettingsRow>
        {#each groups as g (g.label)}
          <SettingsRow label={g.label}>
            <Stack direction="row" gap="1" wrap justify="end">
              {#each g.names as n (n)}<Chip size="sm">{n}</Chip>{/each}
              {#if !g.names.length}<Text tone="tertiary">None</Text>{/if}
            </Stack>
          </SettingsRow>
        {/each}
        <SettingsRow label="Workflows">
          <Stack direction="row" gap="1" wrap justify="end">
            {#each flows as w (w.name)}<Chip size="sm" icon={WorkflowIcon} onclick={w.id ? () => onworkflow?.(w.id) : undefined}>{w.name}</Chip>{/each}
            {#if !flows.length}<Text tone="tertiary">None</Text>{/if}
          </Stack>
        </SettingsRow>
      </Stack>
    {:else if tab === 'columns'}
      <Table columns={columnColumns} rows={columnRows} label="Columns">
        {#snippet cell(row, col)}
          {#if col.key === 'name'}
            <Stack direction="row" gap="2" align="center">
              <Text weight="medium">{row.name}</Text>
              {#if row.to}
                <Chip size="sm" icon={Link} onclick={() => onopen?.(row.to)}>{byId(row.to)?.name ?? row.to}</Chip>
              {/if}
            </Stack>
          {:else if col.key === 'type'}
            <Badge>{typeLabels[row.type]}</Badge>
          {:else if col.key === 'required'}
            {#if row.required}<Icon icon={Check} size="sm" />{/if}
          {:else}
            {row[col.key]}
          {/if}
        {/snippet}
      </Table>
    {:else if tab === 'rows'}
      <Table columns={sampleColumns} rows={type.sample} label="Sample rows" />
    {:else}
      <CodeBlock code={ddl(type)} lang="sql" />
    {/if}
  </div>
</div>

<style>
  .detail {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    flex: 1;
    min-height: 0;
  }

  .pane {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }
</style>
