<script lang="ts">
  import { Select, type MenuItem } from '../../lib';
  import { api } from '../../lib/api';
  import { recordTitle } from '../../lib/spec';
  import { remember } from '../../lib/refs.svelte';
  import { session } from '../../lib/session.svelte';

  let { type, value = $bindable(''), invalid = false, label }: { type: string; value?: string; invalid?: boolean; label?: string } = $props();
  let options = $state<MenuItem[]>([]);

  $effect(() => {
    api
      .list(type, { limit: 500 })
      .then((rs) => {
        rs.forEach(remember);
        options = [
          { id: '', label: 'None', icon: 'x' },
          ...rs.map((r) => ({ id: r.id, label: recordTitle(session.ontology, r) })).sort((a, b) => a.label.localeCompare(b.label))
        ];
      })
      .catch(() => (options = [{ id: '', label: 'None', icon: 'x' }]));
  });
</script>

<Select variant="field" bind:value {options} placeholder={`Choose a ${type}`} searchable {invalid} {label} />
