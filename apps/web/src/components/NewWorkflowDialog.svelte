<script>
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Textarea from './Textarea.svelte';
  import Select from './Select.svelte';
  import Button from './Button.svelte';
  import { triggerKinds } from '../lib/mock/workflows.js';

  let { open = $bindable(false), oncreate } = $props();

  let name = $state('');
  let kind = $state('file');
  let when = $state('');
  let steps = $state('Read the new file\nSummarize it\nSave the summary next to it');

  const hints = { manual: 'Run by hand', schedule: 'Every Monday at 09:00', file: 'When a file is added to Letters', email: 'When an email arrives from the archivist' };

  function create(close) {
    oncreate?.({
      name: name.trim(),
      trigger: { kind, label: when.trim() || hints[kind] },
      steps: steps.split('\n').map((s) => s.trim()).filter(Boolean)
    });
    name = '';
    when = '';
    close();
  }
</script>

<Dialog bind:open title="New workflow">
  <Field label="Name">
    {#snippet children(id)}<Input {id} bind:value={name} placeholder="Summarize new letters" />{/snippet}
  </Field>
  <Field label="Trigger">
    {#snippet children(id)}<Select {id} bind:value={kind} options={triggerKinds} full />{/snippet}
  </Field>
  {#if kind !== 'manual'}
    <Field label="When">
      {#snippet children(id)}<Input {id} bind:value={when} placeholder={hints[kind]} />{/snippet}
    </Field>
  {/if}
  <Field label="Steps, one per line">
    {#snippet children(id)}<Textarea {id} bind:value={steps} rows={4} />{/snippet}
  </Field>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={!name.trim() || !steps.trim()} onclick={() => create(close)}>Create workflow</Button>
  {/snippet}
</Dialog>
