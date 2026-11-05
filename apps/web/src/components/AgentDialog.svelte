<script>
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Textarea from './Textarea.svelte';
  import Button from './Button.svelte';
  import LookPicker from './LookPicker.svelte';
  import Menu from './Menu.svelte';
  import { TINTS, AVATARS } from '../lib/looks.js';
  import { app, groups, members, agentById, createAgent, updateAgent, mayLimit, accessOptions } from '../lib/app.svelte.js';

  groups.use();
  members.use();

  // app.agentDialog is the id of the agent being changed, true for a new one, or null.
  const agent = $derived(agentById(app.agentDialog));
  const open = $derived(Boolean(app.agentDialog));

  let name = $state('');
  let look = $state({ avatar: 0, tint: 0 });
  let instructions = $state('');
  let access = $state('');

  // Only someone who may limit it can choose, and only when there is a group to choose.
  const limits = $derived(mayLimit(agent?.access) && accessOptions(agent?.access).length > 1);

  $effect(() => {
    if (!open) return;
    name = agent?.name ?? '';
    instructions = agent?.instructions ?? '';
    access = agent?.access ?? '';
    look = agent ? { avatar: agent.avatar, tint: agent.tint } : { avatar: Math.floor(Math.random() * AVATARS.length), tint: Math.floor(Math.random() * TINTS) };
  });

  function save(close) {
    const row = { name: name.trim(), avatar: look.avatar, tint: look.tint, instructions, ...(limits && { access: access || null }) };
    if (agent) updateAgent(agent.id, row);
    else createAgent(row);
    close();
  }
</script>

<Dialog bind:open={() => open, (on) => { if (!on) app.agentDialog = null; }} title={agent ? 'Agent settings' : 'New agent'} fit>
  <Field label="Name">
    {#snippet children(id)}
      <Input {id} bind:value={name} />
    {/snippet}
  </Field>
  <LookPicker bind:look agent />
  {#if limits}
    <Field label="Who sees it">
      {#snippet children()}
        <Menu value={access} options={accessOptions(agent?.access)} label="Who sees it" full onchange={(v) => (access = v)} />
      {/snippet}
    </Field>
  {/if}
  <div class="grow">
    <Field label="Instructions">
      {#snippet children(id)}
        <Textarea {id} bind:value={instructions} rows={3} autosize />
      {/snippet}
    </Field>
  </div>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={!name.trim()} onclick={() => save(close)}>{agent ? 'Save' : 'Create agent'}</Button>
  {/snippet}
</Dialog>

<style>
  /* The instructions take what height the popup has left, and past that scroll inside. */
  .grow {
    display: flex;
    flex-direction: column;
    flex: 0 1 auto;
    min-height: 0;
  }

  .grow :global(.field) {
    flex: 0 1 auto;
    min-height: 0;
  }

  .grow :global(textarea) {
    flex: 0 1 auto;
    overflow-y: auto;
  }
</style>
