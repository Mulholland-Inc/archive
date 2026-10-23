<script lang="ts">
  import { Stack, Text } from '../../lib';
  import { humanize } from '../../lib/spec';
  import type { Op } from '../../lib/types';
  let { ops }: { ops: Op[] } = $props();

  function describe(op: Op): string {
    const t = humanize(op.type);
    const def = (op.def ?? {}) as { type?: string; fields?: object };
    switch (op.op) {
      case 'add_type':
        return `Add the ${t} type${def.fields ? ` with ${Object.keys(def.fields).length} fields` : ''}`;
      case 'remove_type':
        return `Remove the ${t} type`;
      case 'change_type':
        return `Change the ${t} type's settings`;
      case 'add_field':
        return `Add ${op.field} to ${t}${def.type ? ` as ${humanize(def.type).toLowerCase()}` : ''}`;
      case 'change_field':
        return `Change ${op.field} on ${t}${def.type ? ` to ${humanize(def.type).toLowerCase()}` : ''}`;
      case 'remove_field':
        return `Remove ${op.field} from ${t}`;
      case 'rename_field':
        return `Rename ${op.field} to ${op.to} on ${t}`;
    }
    return `${op.op} ${op.type}`;
  }
</script>

<Stack gap="1">
  {#each ops as op, i (i)}<Text>{describe(op)}</Text>{/each}
</Stack>
