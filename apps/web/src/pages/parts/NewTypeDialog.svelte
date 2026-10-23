<script lang="ts">
  import { Dialog, Field, Input, Textarea, Button, Stack, Divider, Notice } from '../../lib';
  import { checkName, slug, changeOntology, emptyDraft, draftToField, checkDraft, type FieldDraft } from '../../lib/ontology';
  import { session } from '../../lib/session.svelte';
  import { navigate } from '../../lib/router.svelte';
  import { toast } from '../../lib/toast.svelte';
  import FieldDraftRow from './FieldDraftRow.svelte';

  let { open = $bindable(false) }: { open?: boolean } = $props();

  let label = $state('');
  let name = $state('');
  let nameTouched = $state(false);
  let description = $state('');
  let drafts = $state<FieldDraft[]>([{ ...emptyDraft(), name: 'name', required: true }]);
  let errors = $state<{ name?: string; fields: string[] }>({ fields: [] });
  let busy = $state(false);
  let failure = $state('');

  $effect(() => {
    if (!nameTouched) name = slug(label);
  });

  $effect(() => {
    if (open) {
      label = '';
      name = '';
      nameTouched = false;
      description = '';
      drafts = [{ ...emptyDraft(), name: 'name', required: true }];
      errors = { fields: [] };
      failure = '';
    }
  });

  async function submit(e?: SubmitEvent) {
    e?.preventDefault();
    const taken = Object.keys(session.ontology?.spec.types ?? {});
    const fieldErrors = drafts.map((d, i) => checkDraft(d, drafts.slice(0, i).map((x) => x.name)));
    errors = { name: checkName(name, 'type', taken), fields: fieldErrors };
    if (errors.name || fieldErrors.some(Boolean)) return;
    busy = true;
    failure = '';
    try {
      const fields = Object.fromEntries(drafts.map((d) => [d.name, draftToField(d)]));
      const def: { [k: string]: unknown } = { fields };
      if (description.trim()) def.description = description.trim();
      const { applied } = await changeOntology([{ op: 'add_type', type: name, def }]);
      open = false;
      if (applied) {
        toast(`Created the ${label || name} type`, 'success');
        navigate(`/records/${name}`);
      } else {
        toast('Proposed the new type. An admin needs to apply it under Ontology.', 'info');
        navigate('/changes');
      }
    } catch (err) {
      failure = (err as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<Dialog bind:open title="New type" description="A type is a kind of record, like an invoice or a contact, with the fields it has." size="lg">
  <form onsubmit={submit} novalidate>
    <Stack gap="4">
      {#if failure}<Notice tone="danger">{failure}</Notice>{/if}
      <Field label="Name" hint={name ? `Stored as ${name}` : 'Like Invoice or Contact'} error={errors.name}>
        <Input bind:value={label} placeholder="Invoice" invalid={!!errors.name} autofocus label="Type name" />
      </Field>
      {#if errors.name && label}
        <Field label="Stored name">
          <Input bind:value={name} mono oninput={() => (nameTouched = true)} label="Stored name" />
        </Field>
      {/if}
      <Field label="Description" hint="Helps people and the intake model know what belongs here.">
        <Textarea bind:value={description} rows={2} placeholder="Bills from suppliers, with totals and due dates" label="Description" />
      </Field>
      <Divider label="Fields" />
      {#each drafts as _, i (i)}
        <FieldDraftRow bind:draft={drafts[i]} error={errors.fields[i]} onremove={drafts.length > 1 ? () => drafts.splice(i, 1) : undefined} />
        <Divider />
      {/each}
      <div>
        <Button icon="plus" onclick={() => drafts.push(emptyDraft())}>Add a field</Button>
      </div>
    </Stack>
  </form>
  {#snippet footer()}
    <Button variant="ghost" onclick={() => (open = false)}>Cancel</Button>
    <Button variant="primary" loading={busy} onclick={() => submit()}>Create type</Button>
  {/snippet}
</Dialog>
