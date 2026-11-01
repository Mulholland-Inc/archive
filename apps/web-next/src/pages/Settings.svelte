<script>
  import { User, Palette, Building2, Bell, Sun, Moon, Monitor, Trash2 } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Split from '../components/Split.svelte';
  import VerticalNav from '../components/VerticalNav.svelte';
  import Section from '../components/Section.svelte';
  import Stack from '../components/Stack.svelte';
  import Grid from '../components/Grid.svelte';
  import Field from '../components/Field.svelte';
  import Input from '../components/Input.svelte';
  import Textarea from '../components/Textarea.svelte';
  import Select from '../components/Select.svelte';
  import Switch from '../components/Switch.svelte';
  import SegmentedControl from '../components/SegmentedControl.svelte';
  import ThemePicker from '../components/ThemePicker.svelte';
  import Reveal from '../components/Reveal.svelte';
  import SettingsRow from '../components/SettingsRow.svelte';
  import Avatar from '../components/Avatar.svelte';
  import Button from '../components/Button.svelte';
  import Divider from '../components/Divider.svelte';
  import Dialog from '../components/Dialog.svelte';
  import Text from '../components/Text.svelte';
  import Prose from '../components/Prose.svelte';
  import Card from '../components/Card.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { me } from '../lib/data.js';
  import { theme, setTheme, setChatFont } from '../lib/theme.svelte.js';
  import { app, ws, deleteVault } from '../lib/app.svelte.js';
  import VaultIcon from '../components/VaultIcon.svelte';
  import LookPicker from '../components/LookPicker.svelte';
  import { toast } from '../lib/toast.svelte.js';

  let { section = 'profile' } = $props();

  const nav = [
    { id: 'profile', label: 'Profile', icon: User, href: '#/settings/profile' },
    { id: 'appearance', label: 'Appearance', icon: Palette, href: '#/settings/appearance' },
    { id: 'vault', label: 'Vault', icon: Building2, href: '#/settings/vault' },
    { id: 'notifications', label: 'Notifications', icon: Bell, href: '#/settings/notifications' }
  ];

  let name = $state(app.profile.name);
  let call = $state(app.profile.first);
  let work = $state(app.profile.work);
  let prefs = $state(app.profile.prefs);
  let tone = $state(app.profile.tone);
  let wsName = $state(ws().name);
  let look = $state({ ...ws().look });
  let deleting = $state(false);
  const settings = $derived(ws().settings);
  let chatFont = $state(theme.chatFont);

  const profileDirty = $derived(name !== app.profile.name || call !== app.profile.first || work !== app.profile.work || prefs !== app.profile.prefs || tone !== app.profile.tone);
  const wsDirty = $derived(wsName !== ws().name || look.icon !== ws().look.icon || look.tint !== ws().look.tint || look.type !== ws().look.type);

  $effect(() => {
    app.vault;
    wsName = ws().name;
    look = { ...ws().look };
  });

  function saveProfile() {
    Object.assign(app.profile, { name: name.trim() || app.profile.name, first: call.trim() || app.profile.first, work, prefs, tone });
    toast('Profile saved', { tone: 'success' });
  }

  function resetProfile() {
    name = app.profile.name;
    call = app.profile.first;
    work = app.profile.work;
    prefs = app.profile.prefs;
    tone = app.profile.tone;
  }

  function saveVault() {
    const w = ws();
    w.name = wsName.trim() || w.name;
    w.look = { ...look };
    w.kind = { family: 'Family', business: 'Business', personal: 'Just you' }[look.type];
    toast('Vault saved', { tone: 'success' });
  }

  const works = [
    { value: 'archivist', label: 'Family historian' },
    { value: 'legal', label: 'Legal' },
    { value: 'design', label: 'Design' },
    { value: 'research', label: 'Research' },
    { value: 'other', label: 'Something else' }
  ];
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
  const fonts = [
    { value: 'sans', label: 'Sans' },
    { value: 'serif', label: 'Serif' }
  ];
  const sample = [{ type: 'p', text: 'The postmark reads **ENUGU 18 MR 74**, which matches the date at the top of the second letter.' }];
</script>

<TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <PageHeader title="Settings" />
  <Split side="start">
    {#snippet aside()}<VerticalNav items={nav} active={section} label="Settings" />{/snippet}

    {#key section}
      <Reveal>
        {#if section === 'profile'}
          <Stack gap="8">
            <Section title="Profile" gap="lg">
              <Stack direction="row" gap="4" align="center">
                <Avatar name={name} {tone} size="xl" />
                <Stack direction="row" gap="2">
                  <Button size="sm" onclick={() => (tone = (tone % 6) + 1)}>Change colour</Button>
                  <Button size="sm" variant="ghost" disabled={tone === me.tone} onclick={() => (tone = me.tone)}>Reset</Button>
                </Stack>
              </Stack>
              <Grid columns="2" gap="4">
                <Field label="Full name">{#snippet children(id)}<Input {id} bind:value={name} />{/snippet}</Field>
                <Field label="What should Archive call you?">{#snippet children(id)}<Input {id} bind:value={call} />{/snippet}</Field>
              </Grid>
              <Field label="What best describes your work?">{#snippet children(id)}<Select {id} bind:value={work} options={works} full />{/snippet}</Field>
              <Field label="Personal preferences">{#snippet children(id)}<Textarea {id} bind:value={prefs} rows={4} />{/snippet}</Field>
              <Stack direction="row" gap="2" justify="end">
                <Button variant="ghost" disabled={!profileDirty} onclick={resetProfile}>Cancel</Button>
                <Button variant="primary" disabled={!profileDirty} onclick={saveProfile}>Save changes</Button>
              </Stack>
            </Section>
          </Stack>
        {:else if section === 'appearance'}
          <Stack gap="8">
            <Section title="Theme" gap="lg">
              <ThemePicker value={theme.choice} onchange={setTheme} />
            </Section>
            <Divider />
            <Section title="Chat font" gap="lg">
              <SegmentedControl bind:value={chatFont} options={fonts} label="Chat font" onchange={setChatFont} />
              <Card padding="lg" variant="filled"><Prose blocks={sample} /></Card>
            </Section>
          </Stack>
        {:else if section === 'vault'}
          <Stack gap="8">
            <Section title="Vault" gap="lg">
              <Stack direction="row" gap="4" align="center">
                <VaultIcon icon={look.icon} tint={look.tint} seed={ws().id} size="xxl" />
                <Field label="Name">{#snippet children(id)}<Input {id} bind:value={wsName} />{/snippet}</Field>
              </Stack>
              <LookPicker bind:look />
              <Stack direction="row" gap="2" justify="end">
                <Button variant="ghost" disabled={!wsDirty} onclick={() => { wsName = ws().name; look = { ...ws().look }; }}>Cancel</Button>
                <Button variant="primary" disabled={!wsDirty} onclick={saveVault}>Save changes</Button>
              </Stack>
            </Section>
            <Divider />
            <Section title="Members and data" gap="lg">
              <Stack gap="0">
                <SettingsRow label="Default role for new members"><Select bind:value={settings.defaultRole} options={roles} size="sm" label="Default role" placement="bottom-end" /></SettingsRow>
                <SettingsRow label="Editors can invite people"><Switch bind:checked={settings.invites} label="Editors can invite people" /></SettingsRow>
                <SettingsRow label="Allow public share links"><Switch bind:checked={settings.sharing} label="Allow public share links" /></SettingsRow>
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
        {:else}
          <Section title="Notifications" gap="lg">
            <Stack gap="0">
              <SettingsRow label="When a workflow run finishes"><Switch bind:checked={app.profile.notify.done} label="When a workflow run finishes" /></SettingsRow>
              <SettingsRow label="When someone mentions me"><Switch bind:checked={app.profile.notify.mentions} label="When someone mentions me" /></SettingsRow>
              <SettingsRow label="Weekly digest email"><Switch bind:checked={app.profile.notify.digest} label="Weekly digest email" /></SettingsRow>
              <SettingsRow label="When someone joins the vault"><Switch bind:checked={app.profile.notify.invites} label="When someone joins the vault" /></SettingsRow>
            </Stack>
          </Section>
        {/if}
      </Reveal>
    {/key}
  </Split>
</Page>

<Dialog bind:open={deleting} title="Delete this vault?" size="sm">
  <Text as="p" tone="secondary">All chats, files and workflows in {ws().name} will be deleted for everyone.</Text>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="danger" onclick={() => { close(); deleteVault(ws().id); }}>Delete vault</Button>
  {/snippet}
</Dialog>
