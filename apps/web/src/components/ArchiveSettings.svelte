<script>
  import { Trash2 } from '@lucide/svelte';
  import Stack from './Stack.svelte';
  import Section from './Section.svelte';
  import Divider from './Divider.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import SettingsRow from './SettingsRow.svelte';
  import Button from './Button.svelte';
  import Dialog from './Dialog.svelte';
  import Text from './Text.svelte';
  import ArchiveIcon from './ArchiveIcon.svelte';
  import LookPicker from './LookPicker.svelte';
  import { app, ws, atLeast, deleteArchive, saveArchive } from '../lib/app.svelte.js';

  let name = $state(ws().name);
  let look = $state({ ...ws().look });
  let deleting = $state(false);
  const can = $derived(atLeast('engineer'));
  const dirty = $derived(name !== ws().name || look.icon !== ws().look.icon || look.tint !== ws().look.tint || look.type !== ws().look.type);

  $effect(() => {
    app.archive;
    name = ws().name;
    look = { ...ws().look };
  });

  function save() {
    saveArchive({ name: name.trim() || ws().name, look: { ...look } });
  }

</script>

<Stack gap="8">
  <Stack gap="6">
    <Stack direction="row" gap="4" align="center">
      <ArchiveIcon icon={look.icon} tint={look.tint} seed={ws().id} size="xxl" />
      <Field label="Name">{#snippet children(id)}<Input {id} bind:value={name} disabled={!can} />{/snippet}</Field>
    </Stack>
    {#if can}
      <LookPicker bind:look />
      <Stack direction="row" gap="2" justify="end">
        <Button variant="ghost" disabled={!dirty} onclick={() => { name = ws().name; look = { ...ws().look }; }}>Cancel</Button>
        <Button variant="primary" disabled={!dirty} onclick={save}>Save changes</Button>
      </Stack>
    {/if}
  </Stack>
  {#if app.role === 'owner'}
    <Divider />
    <Section title="Delete archive">
      <SettingsRow label={`Permanently delete ${ws().name}`}>
        <Button variant="danger" icon={Trash2} onclick={() => (deleting = true)}>Delete</Button>
      </SettingsRow>
    </Section>
  {/if}
</Stack>

<Dialog bind:open={deleting} title="Delete this archive?" size="sm">
  <Text as="p" tone="secondary">All agents, files and workflows in {ws().name} will be deleted for everyone.</Text>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="danger" onclick={() => { close(); deleteArchive(ws().id); }}>Delete archive</Button>
  {/snippet}
</Dialog>
