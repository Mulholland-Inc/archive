<script>
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Textarea from './Textarea.svelte';
  import Button from './Button.svelte';

  let { open = $bindable(false), oncreate } = $props();
  let name = $state('');
  let instructions = $state('');

  function create(close) {
    oncreate?.({ name: name.trim(), instructions });
    name = '';
    instructions = '';
    close();
  }
</script>

<Dialog bind:open title="New project">
  <Field label="Name">
    {#snippet children(id)}
      <Input {id} bind:value={name} placeholder="Wedding letters, 1981" />
    {/snippet}
  </Field>
  <Field label="Instructions">
    {#snippet children(id)}
      <Textarea {id} bind:value={instructions} rows={4} placeholder="How should Archive work in this project?" />
    {/snippet}
  </Field>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={!name.trim()} onclick={() => create(close)}>Create project</Button>
  {/snippet}
</Dialog>
