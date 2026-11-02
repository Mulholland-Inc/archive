<script>
  import Dialog from './Dialog.svelte';
  import SettingsRow from './SettingsRow.svelte';
  import Stack from './Stack.svelte';
  import Kbd from './Kbd.svelte';
  import Button from './Button.svelte';

  let { open = $bindable(false) } = $props();
  const keys = [
    ['Search and commands', '⌘ K'],
    ['New chat', '⌘ ⇧ O'],
    ['Toggle sidebar', '⌘ .'],
    ['Settings', '⌘ ,'],
    ['Search the library', '/'],
    ['Send message', '↵'],
    ['New line', '⇧ ↵'],
    ['Close menus and dialogs', 'Esc']
  ];
</script>

<Dialog bind:open title="Keyboard shortcuts">
  <Stack gap="0">
    {#each keys as [label, k] (label)}
      <SettingsRow {label}><Kbd keys={k} /></SettingsRow>
    {/each}
  </Stack>
  {#snippet footer({ close })}
    <Button variant="primary" onclick={close}>Done</Button>
  {/snippet}
</Dialog>
