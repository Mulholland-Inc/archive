<script>
  import Dialog from './Dialog.svelte';
  import Checkbox from './Checkbox.svelte';
  import Avatar from './Avatar.svelte';
  import Stack from './Stack.svelte';
  import Text from './Text.svelte';
  import Button from './Button.svelte';
  import { ws, setGroupMembers } from '../lib/app.svelte.js';
  import { toast } from '../lib/toast.svelte.js';

  let { open = $bindable(false), group } = $props();
  let picked = $state({});

  $effect(() => {
    if (open && group) picked = Object.fromEntries(ws().members.map((m) => [m.id, m.groups.includes(group)]));
  });

  function save(close) {
    setGroupMembers(group, Object.keys(picked).filter((id) => picked[id]));
    toast(`${group} updated`, { tone: 'success' });
    close();
  }
</script>

<Dialog bind:open title={group ?? 'Group'} size="sm">
  <Stack gap="3">
    {#each ws().members as m (m.id)}
      <Checkbox checked={Boolean(picked[m.id])} onchange={(e) => (picked[m.id] = e.currentTarget.checked)}>
        <Stack as="span" direction="row" gap="2" align="center">
          <Avatar name={m.name} tone={m.tone} size="sm" />
          <Text>{m.name}</Text>
          {#if m.status === 'invited'}<Text size="sm" tone="tertiary">Invited</Text>{/if}
        </Stack>
      </Checkbox>
    {/each}
  </Stack>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" onclick={() => save(close)}>Save</Button>
  {/snippet}
</Dialog>
