<script>
  import { Copy, Check } from '@lucide/svelte';
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Textarea from './Textarea.svelte';
  import Button from './Button.svelte';
  import IconButton from './IconButton.svelte';
  import LookPicker from './LookPicker.svelte';
  import Menu from './Menu.svelte';
  import { TINTS, AVATARS } from '../lib/looks.js';
  import { app, groups, members, agentById, agentAddress, createAgent, updateAgent, mayLimit, accessOptions } from '../lib/app.svelte.js';

  groups.use(() => Boolean(app.agentDialog));
  members.use();

  const agent = $derived(agentById(app.agentDialog));
  const open = $derived(Boolean(app.agentDialog));

  let name = $state('');
  let description = $state('');
  let look = $state({ avatar: 0, tint: 0 });
  let instructions = $state('');
  let access = $state('');
  let copied = $state(false);

  const address = $derived(agent ? agentAddress(agent) : '');
  const limits = $derived(mayLimit(agent?.access) && accessOptions(agent?.access).length > 1);

  $effect(() => {
    if (!open) return;
    name = agent?.name ?? '';
    description = agent?.description ?? '';
    instructions = agent?.instructions ?? '';
    access = agent?.access ?? '';
    look = agent ? { avatar: agent.avatar, tint: agent.tint } : { avatar: Math.floor(Math.random() * AVATARS.length), tint: Math.floor(Math.random() * TINTS) };
  });

  async function copy() {
    await navigator.clipboard.writeText(address);
    copied = true;
    setTimeout(() => (copied = false), 1600);
  }

  function save(close) {
    const row = { name: name.trim(), description: description.trim(), avatar: look.avatar, tint: look.tint, instructions, ...(limits && { access: access || null }) };
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
  <Field label="Description">
    {#snippet children(id)}
      <Input {id} bind:value={description} />
    {/snippet}
  </Field>
  {#if agent}
    <Field label="Email">
      {#snippet children(id)}
        <Input {id} value={address} readonly>
          {#snippet trailing()}
            <IconButton icon={copied ? Check : Copy} label={copied ? 'Copied' : 'Copy'} size="xs" tooltip={false} onclick={copy} />
          {/snippet}
        </Input>
      {/snippet}
    </Field>
  {/if}
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
