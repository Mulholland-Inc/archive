<script>
  import { getContext } from 'svelte';
  import Chip from './Chip.svelte';

  let { value = null, fixed = false, onpick } = $props();
  const access = getContext('access');
  const groups = $derived([...new Set([...access.groups, ...(value ? [value] : [])])].sort());
  const can = $derived(onpick && !fixed && access.role !== 'viewer' && groups.length > 0);
</script>

{#if can}
  <select class="access" value={value ?? ''} aria-label="Access" onchange={(e) => onpick(e.currentTarget.value || null)}>
    <option value="">everyone</option>
    {#each groups as g (g)}<option value={g}>{g}</option>{/each}
  </select>
{:else if value}
  <Chip>{value}</Chip>
{/if}

<style>
  .access {
    flex: none;
    padding: var(--space-1);
    border: 0;
    border-radius: var(--radius-xs);
    background: var(--fill);
    color: var(--fg-muted);
    font: inherit;
    font-size: var(--text-meta);
    line-height: var(--leading-none);
    outline: 0;
  }
</style>
