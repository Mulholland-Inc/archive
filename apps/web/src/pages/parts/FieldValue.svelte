<script lang="ts">
  import { Link, Text, Row, Icon } from '../../lib';
  import { formatValue, kindOf, isSecret } from '../../lib/spec';
  import { refTitle } from '../../lib/refs.svelte';
  import { session } from '../../lib/session.svelte';
  import type { Field } from '../../lib/types';

  let { field, value }: { field: Field; value: unknown } = $props();
  const kind = $derived(kindOf(session.ontology, field.type));
  const values = $derived(field.list && Array.isArray(value) ? value : value === undefined || value === null ? [] : [value]);
</script>

{#if !values.length}
  <Text variant="subtle">Empty</Text>
{:else if isSecret(values[0])}
  <Row gap="1-5"><Icon name="lock" size="sm" /><Text variant="muted">Hidden</Text></Row>
{:else if kind === 'reference' && field.to}
  <Row gap="2" wrap>
    {#each values as v (String(v))}
      <Link href={`/records/${encodeURIComponent(field.to)}/${v}`} variant="accent">{refTitle(field.to, String(v))}</Link>
    {/each}
  </Row>
{:else if kind === 'email'}
  <Row gap="2" wrap>{#each values as v (String(v))}<Link href={`mailto:${v}`} external variant="accent">{String(v)}</Link>{/each}</Row>
{:else if kind === 'url'}
  <Row gap="2" wrap>{#each values as v (String(v))}<Link href={String(v)} external variant="accent">{String(v)}</Link>{/each}</Row>
{:else if kind === 'phone'}
  <Row gap="2" wrap>{#each values as v (String(v))}<Link href={`tel:${v}`} external variant="accent">{String(v)}</Link>{/each}</Row>
{:else}
  <Text>{formatValue(session.ontology, field, value)}</Text>
{/if}
