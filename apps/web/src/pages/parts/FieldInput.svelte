<script lang="ts">
  import { Input, Textarea, Select, Checkbox, Row, Text, DatePicker } from '../../lib';
  import { kindOf, humanize } from '../../lib/spec';
  import { session } from '../../lib/session.svelte';
  import type { Field } from '../../lib/types';
  import RecordPicker from './RecordPicker.svelte';

  let {
    name,
    field,
    value = $bindable(''),
    checked = $bindable(false),
    invalid = false,
    secret = false
  }: { name: string; field: Field; value?: string; checked?: boolean; invalid?: boolean; secret?: boolean } = $props();

  const kind = $derived(kindOf(session.ontology, field.type));
  const placeholders: { [k: string]: string } = {
    money: '48.90 EUR',
    timestamp: '2026-10-23 14:30',
    email: 'name@example.com',
    phone: '+31 6 12345678',
    url: 'https://example.com',
    decimal: '12.50',
    integer: '42',
    number: '3.14'
  };
</script>

{#if field.list}
  <Textarea bind:value rows={3} placeholder="One value per line" {invalid} label={humanize(name)} />
{:else if kind === 'boolean'}
  <Row gap="2"><Checkbox {checked} label={humanize(name)} onchange={(v) => (checked = v)} /><Text>{checked ? 'Yes' : 'No'}</Text></Row>
{:else if kind === 'enum' && field.enum}
  <Select
    variant="field"
    bind:value
    options={[{ id: '', label: 'None', icon: 'x' }, ...field.enum.map((v) => ({ id: v, label: humanize(v) }))]}
    placeholder="Choose a value"
    {invalid}
    label={humanize(name)}
  />
{:else if kind === 'reference' && field.to}
  <RecordPicker type={field.to} bind:value {invalid} label={humanize(name)} />
{:else if kind === 'date'}
  <DatePicker bind:value {invalid} label={humanize(name)} />
{:else if kind === 'richtext'}
  <Textarea bind:value rows={4} {invalid} label={humanize(name)} />
{:else}
  <Input
    bind:value
    type={secret ? 'password' : kind === 'email' ? 'email' : 'text'}
    placeholder={placeholders[kind] ?? (kind === 'text' ? '' : typeof session.ontology?.spec.types[field.type]?.examples?.[0] === 'string' ? String(session.ontology?.spec.types[field.type]?.examples?.[0]) : '')}
    mono={['decimal', 'integer', 'number'].includes(kind)}
    {invalid}
    autocomplete="off"
    label={humanize(name)}
  />
{/if}
