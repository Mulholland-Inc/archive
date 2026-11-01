<script>
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Button from './Button.svelte';
  import Text from './Text.svelte';
  import { prompter, confirmer } from '../lib/dialogs.svelte.js';

  function finish(value) {
    prompter.resolve?.(value);
    prompter.resolve = null;
    prompter.open = false;
  }

  function settle(ok) {
    confirmer.resolve?.(ok);
    confirmer.resolve = null;
    confirmer.open = false;
  }

  function submit(event) {
    if (event.key === 'Enter' && prompter.value.trim()) {
      event.preventDefault();
      finish(prompter.value.trim());
    }
  }
</script>

<Dialog bind:open={prompter.open} title={prompter.title} size="sm" onclose={() => finish(null)}>
  <Field label={prompter.label}>
    {#snippet children(id)}
      <Input {id} bind:value={prompter.value} placeholder={prompter.placeholder} onkeydown={submit} />
    {/snippet}
  </Field>
  {#snippet footer()}
    <Button variant="ghost" onclick={() => finish(null)}>Cancel</Button>
    <Button variant="primary" disabled={!prompter.value.trim()} onclick={() => finish(prompter.value.trim())}>{prompter.confirm}</Button>
  {/snippet}
</Dialog>

<Dialog bind:open={confirmer.open} title={confirmer.title} size="sm" onclose={() => settle(false)}>
  {#if confirmer.body}<Text as="p" tone="secondary">{confirmer.body}</Text>{/if}
  {#snippet footer()}
    {#if confirmer.info}
      <Button variant="primary" onclick={() => settle(true)}>{confirmer.confirm}</Button>
    {:else}
      <Button variant="ghost" onclick={() => settle(false)}>Cancel</Button>
      <Button variant="danger" onclick={() => settle(true)}>{confirmer.confirm}</Button>
    {/if}
  {/snippet}
</Dialog>
