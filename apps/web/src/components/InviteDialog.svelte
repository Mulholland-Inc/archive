<script>
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Textarea from './Textarea.svelte';
  import Select from './Select.svelte';
  import Button from './Button.svelte';
  import { invite } from '../lib/app.svelte.js';

  let { open = $bindable(false) } = $props();

  let emails = $state('');
  let role = $state('viewer');
  const list = $derived(emails.split(/[\s,;]+/).filter((e) => /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(e)));
  const count = $derived(list.length);

  const roles = [
    { value: 'viewer', label: 'Viewer' },
    { value: 'editor', label: 'Editor' },
    { value: 'owner', label: 'Owner' }
  ];

  function send(close) {
    invite({ emails: list, role });
    emails = '';
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
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={!count} onclick={() => send(close)}>{count > 1 ? `Send ${count} invites` : 'Send invite'}</Button>
  {/snippet}
</Dialog>
