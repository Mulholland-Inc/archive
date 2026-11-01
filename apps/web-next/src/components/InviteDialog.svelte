<script>
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Textarea from './Textarea.svelte';
  import Select from './Select.svelte';
  import Chip from './Chip.svelte';
  import Button from './Button.svelte';
  import Stack from './Stack.svelte';
  import { ws, invite } from '../lib/app.svelte.js';

  let { open = $bindable(false) } = $props();

  let emails = $state('');
  let role = $state('viewer');
  let picked = $state([]);
  const list = $derived(emails.split(/[\s,;]+/).filter((e) => /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(e)));
  const count = $derived(list.length);

  const roles = [
    { value: 'viewer', label: 'Viewer' },
    { value: 'editor', label: 'Editor' },
    { value: 'owner', label: 'Owner' }
  ];

  function toggle(name) {
    picked = picked.includes(name) ? picked.filter((g) => g !== name) : [...picked, name];
  }

  function send(close) {
    invite({ emails: list, role, groups: picked });
    emails = '';
    picked = [];
    close();
  }
</script>

<Dialog bind:open title="Invite people">
  <Field label="Email addresses">
    {#snippet children(id)}
      <Textarea {id} bind:value={emails} rows={3} placeholder="name@example.com, another@example.com" />
    {/snippet}
  </Field>
  <Field label="Role">
    {#snippet children(id)}
      <Select {id} bind:value={role} options={roles} full />
    {/snippet}
  </Field>
  <Field label="Groups">
    {#snippet children()}
      <Stack direction="row" gap="2" wrap>
        {#each ws().groups as g (g.id)}
          <Chip selected={picked.includes(g.name)} onclick={() => toggle(g.name)}>{g.name}</Chip>
        {/each}
      </Stack>
    {/snippet}
  </Field>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={!count} onclick={() => send(close)}>{count > 1 ? `Send ${count} invites` : 'Send invite'}</Button>
  {/snippet}
</Dialog>
