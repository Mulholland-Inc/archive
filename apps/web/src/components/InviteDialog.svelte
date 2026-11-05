<script>
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Textarea from './Textarea.svelte';
  import Menu from './Menu.svelte';
  import Button from './Button.svelte';
  import Chip from './Chip.svelte';
  import Stack from './Stack.svelte';
  import { Plus } from '@lucide/svelte';
  import { invite, ws, ROLES, atLeast } from '../lib/app.svelte.js';
  import { newGroupPrompt } from '../lib/menus.js';

  let { open = $bindable(false) } = $props();

  let emails = $state('');
  let role = $state('viewer');
  let picked = $state([]);
  const pick = (id) => (picked = picked.includes(id) ? picked.filter((x) => x !== id) : [...picked, id]);

  async function newGroup() {
    const g = await newGroupPrompt();
    if (g) picked = [...picked, g.id];
  }
  const list = $derived(emails.split(/[\s,;]+/).filter((e) => /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(e)));
  const count = $derived(list.length);

  const roles = $derived(ROLES.filter((r) => r.value !== 'owner' && atLeast(r.value)));

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
      <Menu {id} bind:value={role} options={roles} full />
    {/snippet}
  </Field>
  <Field label="Groups">
    {#snippet children()}
      <Stack direction="row" gap="1-5" wrap>
        {#each ws().groups as g (g.id)}<Chip selected={picked.includes(g.id)} onclick={() => pick(g.id)}>{g.name}</Chip>{/each}
        <Chip icon={Plus} onclick={newGroup}>New group</Chip>
      </Stack>
    {/snippet}
  </Field>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={!count} onclick={() => send(close)}>{count > 1 ? `Send ${count} invites` : 'Send invite'}</Button>
  {/snippet}
</Dialog>
