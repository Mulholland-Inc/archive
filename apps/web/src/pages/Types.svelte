<script lang="ts">
  import { Page, PageHeader, Button, Grid, RecordTile, Card, Table, Text, Input, EmptyState, Notice, Stack, Link } from '../lib';
  import { api } from '../lib/api';
  import { session, isAdmin } from '../lib/session.svelte';
  import { navigate } from '../lib/router.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { objectTypes, fieldTypes, fieldsOf, humanize } from '../lib/spec';
  import NewTypeDialog from './parts/NewTypeDialog.svelte';

  setCrumbs([{ label: 'Ontology' }]);

  let creating = $state(false);
  let filter = $state('');
  let pending = $state(0);

  api
    .changes()
    .then((cs) => (pending = (cs ?? []).filter((c) => c.status === 'proposed' && !c.proposed_by.startsWith('revert:')).length))
    .catch(() => {});

  const o = $derived(session.ontology);
  const types = $derived(objectTypes(o));
  const builtin = $derived(
    fieldTypes(o)
      .filter((t) => !filter || t.includes(filter.toLowerCase()) || (o?.spec.types[t]?.description ?? '').toLowerCase().includes(filter.toLowerCase()))
      .map((t) => ({ name: t, def: o!.spec.types[t] }))
  );
</script>

<Page>
  <PageHeader title="Ontology" description={`The types your records use. Version ${o?.spec.version ?? 0}; every change is reviewed, applied and can be reverted.`}>
    {#snippet actions()}
      <Button icon="history" onclick={() => navigate('/changes')}>Changes</Button>
      {#if isAdmin()}<Button variant="primary" icon="plus" onclick={() => (creating = true)}>New type</Button>{/if}
    {/snippet}
  </PageHeader>

  {#if pending}
    <Notice tone="warning">
      {pending === 1 ? 'One change waits' : `${pending} changes wait`} for approval.
      {#snippet action()}<Button variant="ghost" onclick={() => navigate('/changes')}>Review</Button>{/snippet}
    </Notice>
  {/if}

  {#if !types.length}
    <EmptyState icon="shapes" title="No types yet" description="Start with the kinds of things you keep, like invoices, contacts or receipts. You can add fields any time.">
      {#if isAdmin()}<Button variant="primary" icon="plus" onclick={() => (creating = true)}>New type</Button>{/if}
    </EmptyState>
  {:else}
    <Grid min="sm">
      {#each types as t (t)}
        <RecordTile
          href={`/types/${t}`}
          title={humanize(t)}
          rows={[
            { label: 'Fields', value: String(fieldsOf(o, t).length) },
            ...(o?.spec.types[t]?.extends ? [{ label: 'Extends', value: humanize(o.spec.types[t].extends!) }] : []),
            ...(o?.spec.types[t]?.description ? [{ label: 'About', value: o.spec.types[t].description! }] : [])
          ]}
        />
      {/each}
    </Grid>
  {/if}

  <Card title="Built-in field types" description="Each one validates and normalizes its values, so a phone number or an amount is stored the same way everywhere.">
    <Stack gap="3">
      <Input bind:value={filter} type="search" icon="search" placeholder="Filter field types" label="Filter field types" />
      <Table
        columns={[
          { id: 'name', label: 'Type' },
          { id: 'category', label: 'Category' },
          { id: 'description', label: 'Description', width: 'wide' }
        ]}
        rows={builtin}
        key={(r) => r.name}
      >
        {#snippet cell(r, c)}
          {#if c === 'name'}<Text variant="code">{r.name}</Text>
          {:else if c === 'category'}<Text variant="muted">{r.def?.category ? humanize(r.def.category) : ''}</Text>
          {:else}<Text>{r.def?.description ?? ''}</Text>{/if}
        {/snippet}
      </Table>
    </Stack>
  </Card>
</Page>

<NewTypeDialog bind:open={creating} />
