<script lang="ts">
  import { Popover, PopoverCard, Menu, Button, Input, Stack, Row, Text, type MenuItem } from '../../lib';
  import { fieldsOf, humanize, kindOf } from '../../lib/spec';
  import { session } from '../../lib/session.svelte';

  let { type, where, onadd }: { type: string; where: { [k: string]: string }; onadd: (field: string, value: string) => void } = $props();

  let open = $state(false);
  let field = $state<string | null>(null);
  let text = $state('');

  const fields = $derived(fieldsOf(session.ontology, type).filter(([n, f]) => !where[n] && !f.list));
  const current = $derived(field ? fieldsOf(session.ontology, type).find(([n]) => n === field)?.[1] : undefined);
  const kind = $derived(current ? kindOf(session.ontology, current.type) : 'text');

  const fieldItems = $derived<MenuItem[]>(fields.map(([n, f]) => ({ id: n, label: humanize(n), trailing: humanize(f.type) })));
  const valueItems = $derived<MenuItem[]>(
    kind === 'boolean'
      ? [
          { id: 'true', label: 'Yes' },
          { id: 'false', label: 'No' }
        ]
      : (current?.enum ?? []).map((v) => ({ id: v, label: humanize(v) }))
  );

  function reset() {
    field = null;
    text = '';
  }

  function add(value: string) {
    if (!field || !value.trim()) return;
    onadd(field, value.trim());
    open = false;
    reset();
  }
</script>

<Popover bind:open onclose={reset}>
  {#snippet trigger({ toggle, open })}
    <Button variant="ghost" icon="filter" pressed={open} onclick={toggle} disabled={!fields.length}>Filter</Button>
  {/snippet}
  {#snippet children()}
    {#if !field}
      <Menu items={fieldItems} placeholder="Filter by…" onselect={(i) => (field = i.id)} />
    {:else if valueItems.length}
      <Menu items={valueItems} placeholder={`${humanize(field)} is…`} onselect={(i) => add(i.id)} />
    {:else}
      <PopoverCard>
      <form
        onsubmit={(e) => {
          e.preventDefault();
          add(text);
        }}
      >
        <Stack gap="2">
          <Text variant="small">{humanize(field)} is</Text>
          <Input bind:value={text} autofocus placeholder={kind === 'date' ? '2026-10-23' : kind === 'money' ? '48.90 EUR' : 'Value'} label="Value" />
          <Row justify="end" gap="2">
            <Button variant="ghost" onclick={reset}>Back</Button>
            <Button variant="primary" type="submit">Apply</Button>
          </Row>
        </Stack>
      </form>
      </PopoverCard>
    {/if}
  {/snippet}
</Popover>
