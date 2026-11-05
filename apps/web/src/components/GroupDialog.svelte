<script>
  import { untrack } from 'svelte';
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Textarea from './Textarea.svelte';
  import PeoplePicker from './PeoplePicker.svelte';
  import Button from './Button.svelte';
  import { ws, createGroup } from '../lib/app.svelte.js';
  import { go } from '../lib/router.svelte.js';

  let { open = $bindable(false), people: start = [] } = $props();

  let name = $state('');
  let description = $state('');
  let picked = $state([]);
  let saving = $state(false);

  const everyone = $derived(ws().members);
  const clean = $derived(name.trim().toLowerCase());
  const taken = $derived(ws().groups.some((g) => g.name === clean));
  const valid = $derived(/^[a-z0-9][a-z0-9_]{0,23}$/.test(clean));
  const error = $derived(!clean ? '' : taken ? 'There is already a group by that name' : valid ? '' : 'Use 1-24 lowercase letters, digits or _, like finance');

  $effect.pre(() => {
    if (!open) return;
    untrack(() => {
      name = '';
      description = '';
      picked = everyone.filter((m) => start.includes(m.id));
    });
  });

  async function save(close) {
    saving = true;
    const g = await createGroup({ name: clean, description: description.trim(), people: picked.map((p) => p.id) });
    saving = false;
    if (!g) return;
    close();
    go(`/access/groups/${g.id}`);
  }
</script>

<Dialog bind:open title="New group">
  <Field label="Name" {error}>
    {#snippet children(id)}<Input {id} bind:value={name} placeholder="finance" maxlength="24" autocomplete="off" invalid={Boolean(error)} />{/snippet}
  </Field>
  <Field label="Description">
    {#snippet children(id)}<Textarea {id} bind:value={description} rows={2} placeholder="What it is for, or who is in it" />{/snippet}
  </Field>
  <Field label="Members">
    {#snippet children()}<PeoplePicker bind:value={picked} people={everyone} label="Members" placeholder="Add members by name or email" />{/snippet}
  </Field>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" loading={saving} disabled={!valid || taken} onclick={() => save(close)}>Create group</Button>
  {/snippet}
</Dialog>
