<script>
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Button from './Button.svelte';
  import LookPicker from './LookPicker.svelte';
  import { defaultLook } from '../lib/looks.js';

  let { open = $bindable(false), oncreate } = $props();
  let name = $state('');
  let look = $state(defaultLook('family'));

  function create(close) {
    oncreate?.(name.trim(), { ...look });
    name = '';
    look = defaultLook('family');
    close();
  }
</script>

<Dialog bind:open title="New workspace">
  <Field label="Name">
    {#snippet children(id)}
      <Input {id} bind:value={name} placeholder="Nwosu Family Archive" onkeydown={(e) => e.key === 'Enter' && name.trim() && create(() => (open = false))} />
    {/snippet}
  </Field>
  <LookPicker bind:look />
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={!name.trim()} onclick={() => create(close)}>Create workspace</Button>
  {/snippet}
</Dialog>
