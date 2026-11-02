<script>
  import { Trash2 } from '@lucide/svelte';
  import Stack from './Stack.svelte';
  import Section from './Section.svelte';
  import Divider from './Divider.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Select from './Select.svelte';
  import Switch from './Switch.svelte';
  import SettingsRow from './SettingsRow.svelte';
  import Button from './Button.svelte';
  import Dialog from './Dialog.svelte';
  import Text from './Text.svelte';
  import VaultIcon from './VaultIcon.svelte';
  import LookPicker from './LookPicker.svelte';
  import { app, ws, deleteVault, saveVault } from '../lib/app.svelte.js';

  let name = $state(ws().name);
  let look = $state({ ...ws().look });
  let deleting = $state(false);
  const settings = $derived(ws().settings);
  const dirty = $derived(name !== ws().name || look.icon !== ws().look.icon || look.tint !== ws().look.tint || look.type !== ws().look.type);

  $effect(() => {
    app.vault;
    name = ws().name;
    look = { ...ws().look };
  });

  function save() {
    saveVault({ name: name.trim() || ws().name, look: { ...look } });
  }

  const roles = [
    { value: 'viewer', label: 'Viewer' },
    { value: 'editor', label: 'Editor' }
  ];
  const regions = [
    { value: 'us-central1', label: 'United States (Iowa)' },
    { value: 'us-east4', label: 'United States (Virginia)' },
    { value: 'us-west1', label: 'United States (Oregon)' }
  ];
  const retentions = [
    { value: 'forever', label: 'Keep forever' },
    { value: '1y', label: '1 year' },
    { value: '90d', label: '90 days' }
  ];
</script>

<Stack gap="8">
  <Stack gap="6">
    <Stack direction="row" gap="4" align="center">
      <VaultIcon icon={look.icon} tint={look.tint} seed={ws().id} size="xxl" />
      <Field label="Name">{#snippet children(id)}<Input {id} bind:value={name} />{/snippet}</Field>
    </Stack>
    <LookPicker bind:look />
    <Stack direction="row" gap="2" justify="end">
      <Button variant="ghost" disabled={!dirty} onclick={() => { name = ws().name; look = { ...ws().look }; }}>Cancel</Button>
      <Button variant="primary" disabled={!dirty} onclick={save}>Save changes</Button>
    </Stack>
  </Stack>
  <Divider />
  <Section title="Members" gap="lg">
    <Stack gap="0">
      <SettingsRow label="Default role for new members"><Select bind:value={settings.defaultRole} options={roles} size="sm" label="Default role" placement="bottom-end" /></SettingsRow>
      <SettingsRow label="Editors can invite people"><Switch bind:checked={settings.invites} label="Editors can invite people" /></SettingsRow>
    </Stack>
  </Section>
  <Divider />
  <Section title="Data" gap="lg">
    <Stack gap="0">
      <SettingsRow label="Data region"><Select bind:value={settings.region} options={regions} size="sm" label="Data region" placement="bottom-end" /></SettingsRow>
      <SettingsRow label="Chat history"><Select bind:value={settings.retention} options={retentions} size="sm" label="Chat history" placement="bottom-end" /></SettingsRow>
    </Stack>
  </Section>
  <Divider />
  <Section title="Delete vault">
    <SettingsRow label={`Permanently delete ${ws().name}`}>
      <Button variant="danger" icon={Trash2} onclick={() => (deleting = true)}>Delete</Button>
    </SettingsRow>
  </Section>
</Stack>

<Dialog bind:open={deleting} title="Delete this vault?" size="sm">
  <Text as="p" tone="secondary">All chats, files and workflows in {ws().name} will be deleted for everyone.</Text>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="danger" onclick={() => { close(); deleteVault(ws().id); }}>Delete vault</Button>
  {/snippet}
</Dialog>
