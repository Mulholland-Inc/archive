<script>
  import Stack from './Stack.svelte';
  import Switch from './Switch.svelte';
  import SettingsRow from './SettingsRow.svelte';
  import { app } from '../lib/app.svelte.js';
</script>

<Stack gap="0">
  <SettingsRow label="When a workflow run finishes"><Switch bind:checked={app.profile.notify.done} label="When a workflow run finishes" /></SettingsRow>
  <SettingsRow label="When someone mentions me"><Switch bind:checked={app.profile.notify.mentions} label="When someone mentions me" /></SettingsRow>
  <SettingsRow label="Weekly digest email"><Switch bind:checked={app.profile.notify.digest} label="Weekly digest email" /></SettingsRow>
  <SettingsRow label="When someone joins the vault"><Switch bind:checked={app.profile.notify.invites} label="When someone joins the vault" /></SettingsRow>
</Stack>
