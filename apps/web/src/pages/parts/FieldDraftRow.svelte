<script lang="ts">
  import { Field, Input, Select, Checkbox, Row, IconButton, Stack, Text, Columns } from '../../lib';
  import { fieldTypeItems, objectTypeItems, type FieldDraft } from '../../lib/ontology';

  let { draft = $bindable(), error, onremove, autofocus = false }: { draft: FieldDraft; error?: string; onremove?: () => void; autofocus?: boolean } = $props();
</script>

<Stack gap="2">
  <Columns trailing={!!onremove}>
    <Field label="Field name">
      <Input bind:value={draft.name} placeholder="total" mono invalid={!!error} {autofocus} label="Field name" />
    </Field>
    <Field label="Type">
      <Select variant="field" bind:value={draft.type} options={fieldTypeItems()} searchable label="Field type" />
    </Field>
    {#if onremove}<IconButton icon="trash" label="Remove field" onclick={onremove} />{/if}
  </Columns>
  {#if draft.type === 'reference'}
    <Field label="Points to">
      <Select variant="field" bind:value={draft.to} options={objectTypeItems()} placeholder="Choose a type" label="Points to" />
    </Field>
  {/if}
  {#if draft.type === 'enum'}
    <Field label="Allowed values" hint="Separate values with commas, like draft, sent, paid.">
      <Input bind:value={draft.values} placeholder="draft, sent, paid" label="Allowed values" />
    </Field>
  {/if}
  <Row gap="6">
    <Row gap="2"><Checkbox checked={draft.required} label="Required" onchange={(v) => (draft.required = v)} /><Text>Required</Text></Row>
    <Row gap="2"><Checkbox checked={draft.list} label="List of values" onchange={(v) => (draft.list = v)} /><Text>List of values</Text></Row>
  </Row>
  {#if error}<Text tone="danger">{error}</Text>{/if}
</Stack>
