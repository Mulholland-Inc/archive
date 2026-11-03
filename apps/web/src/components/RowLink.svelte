<script>
  import Chip from './Chip.svelte';
  import { linkTitle, linkHref } from '../lib/app.svelte.js';

  // A link to a row, as a chip with the row's name that opens its page: one id, or each of a list of them.
  let { table, value, icon, size = 'sm' } = $props();

  const ids = $derived([...new Set((Array.isArray(value) ? value : [value]).filter((v) => typeof v === 'string' && v))]);
</script>

{#each ids as id (id)}<Chip {size} {icon} href={linkHref(table, id)}>{linkTitle(table, id)}</Chip>{/each}
