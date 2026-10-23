<script lang="ts">
  import { Dialog, Field, Button, Stack, Notice, Select } from '../../lib';
  import { api, ApiError } from '../../lib/api';
  import { fieldsOf, humanize, inputValue, parseInput, kindOf, isSecret } from '../../lib/spec';
  import { session, spaceLabel } from '../../lib/session.svelte';
  import { remember } from '../../lib/refs.svelte';
  import type { ArchiveRecord } from '../../lib/types';
  import FieldInput from './FieldInput.svelte';

  let {
    open = $bindable(false),
    type,
    record,
    preset = {},
    onsaved
  }: {
    open?: boolean;
    type: string;
    record?: ArchiveRecord;
    preset?: { [field: string]: string };
    onsaved?: (r: ArchiveRecord) => void;
  } = $props();

  const o = $derived(session.ontology);
  const fields = $derived(fieldsOf(o, type));
  const secretTypes = $derived(new Set(Object.entries(o?.spec.types ?? {}).filter(([, t]) => t.sensitivity === 'secret').map(([n]) => n)));
  const writable = $derived(session.spaces.filter((s) => ['editor', 'admin', 'owner'].includes(s.level)));

  let values = $state<{ [k: string]: string }>({});
  let checks = $state<{ [k: string]: boolean }>({});
  let original = $state<{ [k: string]: string | boolean }>({});
  let errors = $state<{ [k: string]: string }>({});
  let space = $state('');
  let busy = $state(false);
  let failure = $state('');
  let ready = $state(false);

  $effect(() => {
    if (!open) {
      ready = false;
      return;
    }
    const v: { [k: string]: string } = {};
    const c: { [k: string]: boolean } = {};
    const orig: { [k: string]: string | boolean } = {};
    for (const [name, f] of fields) {
      const raw = record?.data[name];
      c[name] = false;
      v[name] = '';
      if (kindOf(o, f.type) === 'boolean' && !f.list) {
        c[name] = raw === true || (!record && preset[name] === 'true');
        orig[name] = record ? c[name] : false;
      } else {
        v[name] = record ? inputValue(o, f, raw) : (preset[name] ?? '');
        orig[name] = record ? v[name] : '';
      }
    }
    values = v;
    checks = c;
    original = orig;
    errors = {};
    failure = '';
    const preferred = writable.find((s) => s.name === session.space) ?? writable.find((s) => s.id === session.home) ?? writable[0];
    space = preferred?.name ?? '';
    ready = true;
  });

  async function submit(e?: SubmitEvent) {
    e?.preventDefault();
    busy = true;
    errors = {};
    failure = '';
    const data: { [k: string]: unknown } = {};
    for (const [name, f] of fields) {
      const isBool = kindOf(o, f.type) === 'boolean' && !f.list;
      const now = isBool ? checks[name] : values[name];
      if (record) {
        if (now === original[name]) continue;
        if (!isBool && now === '' && isSecret(record.data[name])) continue;
      } else if (!isBool && now === '') continue;
      else if (isBool && !now && !f.required) continue;
      data[name] = isBool ? now : parseInput(o, f, now as string);
    }
    try {
      let saved: ArchiveRecord;
      if (record) {
        if (!Object.keys(data).length) {
          open = false;
          return;
        }
        saved = await api.patch(record.type, record.id, data);
      } else {
        saved = await api.create(type, data, space || undefined);
      }
      remember(saved);
      open = false;
      onsaved?.(saved);
    } catch (err) {
      if (err instanceof ApiError && err.fields.length) {
        for (const f of err.fields) {
          const key = f.path.split(/[.[]/)[0];
          if (fields.some(([n]) => n === key)) errors[key] = f.message;
          else failure = `${f.path}: ${f.message}`;
        }
      } else failure = (err as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<Dialog bind:open title={record ? `Edit ${humanize(type).toLowerCase()}` : `New ${humanize(type).toLowerCase()}`} size="lg">
  <form onsubmit={submit} novalidate>
    <Stack gap="4">
      {#if failure}<Notice tone="danger">{failure}</Notice>{/if}
      {#if !record && writable.length > 1}
        <Field label="Space">
          <Select variant="field" bind:value={space} options={writable.map((s) => ({ id: s.name, label: spaceLabel(s.name), icon: 'folder' }))} label="Space" />
        </Field>
      {/if}
      {#if ready}{#each fields as [name, f] (name)}
        <Field
          label={humanize(name)}
          required={f.required}
          error={errors[name]}
          hint={f.description ?? (record && isSecret(record.data[name]) ? 'Hidden. Leave it empty to keep the current value.' : undefined)}
        >
          <FieldInput {name} field={f} bind:value={values[name]} bind:checked={checks[name]} invalid={!!errors[name]} secret={secretTypes.has(f.type)} />
        </Field>
      {/each}{/if}
      <button type="submit" hidden aria-hidden="true" tabindex="-1"></button>
    </Stack>
  </form>
  {#snippet footer()}
    <Button variant="ghost" onclick={() => (open = false)}>Cancel</Button>
    <Button variant="primary" loading={busy} onclick={() => submit()}>{record ? 'Save' : 'Create'}</Button>
  {/snippet}
</Dialog>
