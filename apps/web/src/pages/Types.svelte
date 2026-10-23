<script lang="ts">
  import { Page, PageHeader, Button, Grid, RecordTile, EmptyState, Notice } from '../lib';
  import { api } from '../lib/api';
  import { session, isAdmin } from '../lib/session.svelte';
  import { navigate } from '../lib/router.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { objectTypes, fieldsOf, humanize, typeIcon } from '../lib/spec';
  import NewTypeDialog from './parts/NewTypeDialog.svelte';

  setCrumbs([{ label: 'Ontology' }]);

  let creating = $state(false);
  let pending = $state(0);

  api
    .changes()
    .then((cs) => (pending = (cs ?? []).filter((c) => c.status === 'proposed' && !c.proposed_by.startsWith('revert:')).length))
    .catch(() => {});

  const o = $derived(session.ontology);
  const types = $derived(objectTypes(o));
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
          icon={typeIcon(o, t)}
          rows={[
            { label: 'Fields', value: String(fieldsOf(o, t).length) },
            ...(o?.spec.types[t]?.extends ? [{ label: 'Extends', value: humanize(o.spec.types[t].extends!) }] : [])
          ]}
        />
      {/each}
    </Grid>
  {/if}
</Page>

<NewTypeDialog bind:open={creating} />
