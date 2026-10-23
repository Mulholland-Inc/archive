<script lang="ts">
  import { Page, PageHeader, Button, IconButton, Table, Text, Row, Dialog, Field, Input, Textarea, Stack, Notice, EmptyState, Popover, Menu, Link } from '../lib';
  import { session, isAdmin } from '../lib/session.svelte';
  import { navigate } from '../lib/router.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { fieldsOf, humanize } from '../lib/spec';
  import { changeOntology, emptyDraft, draftToField, checkDraft, checkName, type FieldDraft } from '../lib/ontology';
  import { toast, fail } from '../lib/toast.svelte';
  import type { Op } from '../lib/types';
  import FieldDraftRow from './parts/FieldDraftRow.svelte';

  let { name }: { name: string } = $props();

  $effect(() => setCrumbs([{ label: 'Ontology', href: '/types' }, { label: humanize(name) }]));

  const o = $derived(session.ontology);
  const def = $derived(o?.spec.types[name]);
  const own = $derived(def?.fields ?? {});
  const fields = $derived(fieldsOf(o, name));
  const builtin = $derived(!!o?.builtin.includes(name));
  const admin = $derived(isAdmin() && !builtin);

  let adding = $state(false);
  let draft = $state<FieldDraft>(emptyDraft());
  let draftError = $state('');
  let renaming = $state<string | null>(null);
  let newName = $state('');
  let renameError = $state('');
  let removing = $state<string | null>(null);
  let describing = $state(false);
  let description = $state('');
  let deleting = $state(false);
  let busy = $state(false);
  let menus = $state<{ [k: string]: boolean }>({});
  let typeMenu = $state(false);

  async function run(ops: Op[], done: string) {
    busy = true;
    try {
      const { applied } = await changeOntology(ops);
      toast(applied ? done : 'Proposed. An admin needs to apply it under Changes.', applied ? 'success' : 'info');
      return true;
    } catch (e) {
      fail(e);
      return false;
    } finally {
      busy = false;
    }
  }

  async function addField(e?: SubmitEvent) {
    e?.preventDefault();
    draftError = checkDraft(draft, fields.map(([n]) => n));
    if (draftError) return;
    if (await run([{ op: 'add_field', type: name, field: draft.name, def: draftToField(draft) }], `Added ${draft.name}`)) adding = false;
  }

  async function rename() {
    if (!renaming) return;
    renameError = checkName(newName, 'field', fields.map(([n]) => n));
    if (renameError) return;
    if (await run([{ op: 'rename_field', type: name, field: renaming, to: newName }], `Renamed ${renaming} to ${newName}`)) renaming = null;
  }

  async function removeField() {
    if (!removing) return;
    if (await run([{ op: 'remove_field', type: name, field: removing }], `Removed ${removing}`)) removing = null;
  }

  async function saveDescription() {
    const { fields: _f, ...rest } = def ?? {};
    if (await run([{ op: 'change_type', type: name, def: { ...rest, description: description.trim() || undefined } }], 'Saved the description')) describing = false;
  }

  async function deleteType() {
    if (await run([{ op: 'remove_type', type: name }], `Removed the ${humanize(name)} type`)) {
      deleting = false;
      navigate('/types');
    }
  }
</script>

{#if !def}
  <Page>
    <EmptyState icon="shapes" title={`There is no ${name} type`} description="It may have been removed or renamed.">
      <Button onclick={() => navigate('/types')}>All types</Button>
    </EmptyState>
  </Page>
{:else}
  <Page>
    <PageHeader title={humanize(name)} description={def.description ?? (builtin ? 'A built-in field type.' : 'No description yet.')}>
      {#snippet meta()}
        <Text variant="mono">{name}</Text>
        {#if def.extends}<Text>Extends <Link href={`/types/${def.extends}`} variant="accent">{humanize(def.extends)}</Link></Text>{/if}
        {#if def.sensitivity}<Text>{humanize(def.sensitivity)}</Text>{/if}
      {/snippet}
      {#snippet actions()}
        {#if !builtin}<Button icon="table" onclick={() => navigate(`/records/${name}`)}>Records</Button>{/if}
        {#if admin}
          <Button variant="primary" icon="plus" onclick={() => { draft = emptyDraft(); draftError = ''; adding = true; }}>Add field</Button>
          <Popover bind:open={typeMenu} align="end">
            {#snippet trigger({ toggle })}<IconButton icon="more" label="Type options" onclick={toggle} />{/snippet}
            {#snippet children({ close })}
              <Menu
                searchable={false}
                items={[
                  { id: 'describe', label: 'Edit description', icon: 'pencil' },
                  { id: 'delete', label: 'Remove type', icon: 'trash', danger: true, separator: true }
                ]}
                onselect={(i) => {
                  close();
                  if (i.id === 'describe') {
                    description = def?.description ?? '';
                    describing = true;
                  }
                  if (i.id === 'delete') deleting = true;
                }}
              />
            {/snippet}
          </Popover>
        {/if}
      {/snippet}
    </PageHeader>

    {#if fields.length}
      <Table
        columns={[
          { id: 'name', label: 'Field' },
          { id: 'type', label: 'Type' },
          { id: 'rules', label: 'Rules', width: 'wide' },
          { id: 'actions', label: '', width: 'narrow' }
        ]}
        rows={fields}
        key={([n]) => n}
      >
        {#snippet cell([n, f], c)}
          {#if c === 'name'}
            <Text variant="code">{n}</Text>
          {:else if c === 'type'}
            <Link href={`/types/${f.type}`} variant="accent">{humanize(f.type)}</Link>
          {:else if c === 'rules'}
            <Text variant="muted">
              {[
                f.required ? 'Required' : '',
                f.list ? 'List' : '',
                f.indexed ? 'Indexed' : '',
                f.to ? `Points to ${humanize(f.to)}` : '',
                f.enum ? f.enum.join(', ') : '',
                own[n] ? '' : 'Inherited'
              ]
                .filter(Boolean)
                .join(', ')}
            </Text>
          {:else if c === 'actions' && admin && own[n]}
            <Popover bind:open={menus[n]} align="end">
              {#snippet trigger({ toggle })}<IconButton icon="more" label={`Options for ${n}`} onclick={toggle} />{/snippet}
              {#snippet children({ close })}
                <Menu
                  searchable={false}
                  items={[
                    { id: 'rename', label: 'Rename', icon: 'pencil' },
                    { id: 'remove', label: 'Remove', icon: 'trash', danger: true, separator: true }
                  ]}
                  onselect={(i) => {
                    close();
                    if (i.id === 'rename') {
                      renaming = n;
                      newName = n;
                      renameError = '';
                    }
                    if (i.id === 'remove') removing = n;
                  }}
                />
              {/snippet}
            </Popover>
          {/if}
        {/snippet}
      </Table>
    {:else if builtin}
      <Notice>This field type stores a single value{def.examples?.length ? `, like ${JSON.stringify(def.examples[0])}` : ''}.</Notice>
    {:else}
      <EmptyState icon="plus" title="No fields yet" description="Add the fields records of this type have.">
        {#if admin}<Button variant="primary" icon="plus" onclick={() => (adding = true)}>Add field</Button>{/if}
      </EmptyState>
    {/if}
  </Page>

  <Dialog bind:open={adding} title={`Add a field to ${humanize(name).toLowerCase()}`} description="Existing records keep their data; a required field makes records without it invalid until filled in.">
    <form onsubmit={addField} novalidate>
      <FieldDraftRow bind:draft error={draftError} autofocus />
    </form>
    {#snippet footer()}
      <Button variant="ghost" onclick={() => (adding = false)}>Cancel</Button>
      <Button variant="primary" loading={busy} onclick={() => addField()}>Add field</Button>
    {/snippet}
  </Dialog>

  <Dialog open={!!renaming} title={`Rename ${renaming ?? ''}`} description="Existing records move their values to the new name." onclose={() => (renaming = null)}>
    <Field label="New name" error={renameError}>
      <Input bind:value={newName} mono invalid={!!renameError} autofocus label="New name" />
    </Field>
    {#snippet footer()}
      <Button variant="ghost" onclick={() => (renaming = null)}>Cancel</Button>
      <Button variant="primary" loading={busy} onclick={rename}>Rename</Button>
    {/snippet}
  </Dialog>

  <Dialog open={!!removing} title={`Remove ${removing ?? ''}?`} description="Its values go from every record of this type. You can revert the change afterwards." onclose={() => (removing = null)}>
    {#snippet children()}{/snippet}
    {#snippet footer()}
      <Button variant="ghost" onclick={() => (removing = null)}>Cancel</Button>
      <Button variant="danger" loading={busy} onclick={removeField}>Remove field</Button>
    {/snippet}
  </Dialog>

  <Dialog bind:open={describing} title="Edit description">
    <Field label="Description" hint="Helps people and the intake model know what belongs here.">
      <Textarea bind:value={description} rows={3} label="Description" />
    </Field>
    {#snippet footer()}
      <Button variant="ghost" onclick={() => (describing = false)}>Cancel</Button>
      <Button variant="primary" loading={busy} onclick={saveDescription}>Save</Button>
    {/snippet}
  </Dialog>

  <Dialog bind:open={deleting} title={`Remove the ${humanize(name)} type?`} description="Its records stay in the database but no longer show up until you revert this change.">
    {#snippet children()}{/snippet}
    {#snippet footer()}
      <Button variant="ghost" onclick={() => (deleting = false)}>Cancel</Button>
      <Button variant="danger" loading={busy} onclick={deleteType}>Remove type</Button>
    {/snippet}
  </Dialog>
{/if}
