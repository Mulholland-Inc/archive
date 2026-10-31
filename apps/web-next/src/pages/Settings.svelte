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
  import { currentWorkspace } from '../lib/app.svelte.js';
  import { toast } from '../lib/toast.svelte.js';

  let { section = 'profile' } = $props();

  const nav = [
    { id: 'profile', label: 'Profile', icon: User, href: '#/settings/profile' },
    { id: 'appearance', label: 'Appearance', icon: Palette, href: '#/settings/appearance' },
    { id: 'workspace', label: 'Workspace', icon: Building2, href: '#/settings/workspace' },
    { id: 'notifications', label: 'Notifications', icon: Bell, href: '#/settings/notifications' }
  ];

  let name = $state(me.name);
  let call = $state(me.first);
  let work = $state('archivist');
  let prefs = $state('Keep answers short. Use British spelling. When you quote a letter, keep the original spelling and punctuation.');
  let wsName = $state(currentWorkspace().name);
  let defaultRole = $state('viewer');
  let region = $state('us-central1');
  let invites = $state(true);
  let sharing = $state(false);
  let retention = $state('forever');
  let deleting = $state(false);
  let notify = $state({ done: true, mentions: true, digest: true, invites: false });
  let chatFont = $state(theme.chatFont);

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

<TopBar>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
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
                <Avatar name={name} tone={me.tone} size="xl" />
                <Stack direction="row" gap="2">
                  <Button size="sm">Change photo</Button>
                  <Button size="sm" variant="ghost">Remove</Button>
                </Stack>
              </Stack>
              <Grid columns="2" gap="4">
                <Field label="Full name">{#snippet children(id)}<Input {id} bind:value={name} />{/snippet}</Field>
                <Field label="What should Archive call you?">{#snippet children(id)}<Input {id} bind:value={call} />{/snippet}</Field>
              </Grid>
              <Field label="What best describes your work?">{#snippet children(id)}<Select {id} bind:value={work} options={works} full />{/snippet}</Field>
              <Field label="Personal preferences">{#snippet children(id)}<Textarea {id} bind:value={prefs} rows={4} />{/snippet}</Field>
              <Stack direction="row" gap="2" justify="end">
                <Button variant="ghost" onclick={() => { name = me.name; call = me.first; }}>Cancel</Button>
                <Button variant="primary" onclick={() => toast('Profile saved', { tone: 'success' })}>Save changes</Button>
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
        {:else if section === 'workspace'}
          <Stack gap="8">
            <Section title="Workspace" gap="lg">
              <Stack direction="row" gap="4" align="center">
                <Avatar name={wsName} tone={currentWorkspace().tone} square size="xl" />
                <Button size="sm">Change icon</Button>
              </Stack>
              <Field label="Name">{#snippet children(id)}<Input {id} bind:value={wsName} />{/snippet}</Field>
              <Stack gap="0">
                <SettingsRow label="Default role for new members"><Select bind:value={defaultRole} options={roles} size="sm" label="Default role" placement="bottom-end" /></SettingsRow>
                <SettingsRow label="Editors can invite people"><Switch bind:checked={invites} label="Editors can invite people" /></SettingsRow>
                <SettingsRow label="Allow public share links"><Switch bind:checked={sharing} label="Allow public share links" /></SettingsRow>
                <SettingsRow label="Data region"><Select bind:value={region} options={regions} size="sm" label="Data region" placement="bottom-end" /></SettingsRow>
                <SettingsRow label="Chat history"><Select bind:value={retention} options={retentions} size="sm" label="Chat history" placement="bottom-end" /></SettingsRow>
              </Stack>
              <Stack direction="row" justify="end">
                <Button variant="primary" onclick={() => toast('Workspace saved', { tone: 'success' })}>Save changes</Button>
              </Stack>
            </Section>
            <Divider />
            <Section title="Delete workspace">
              <SettingsRow label={`Permanently delete ${wsName}`}>
                <Button variant="danger" icon={Trash2} onclick={() => (deleting = true)}>Delete</Button>
              </SettingsRow>
            </Section>
          </Stack>
        {:else}
          <Section title="Notifications" gap="lg">
            <Stack gap="0">
              <SettingsRow label="When a task finishes"><Switch bind:checked={notify.done} label="When a task finishes" /></SettingsRow>
              <SettingsRow label="When someone mentions me"><Switch bind:checked={notify.mentions} label="When someone mentions me" /></SettingsRow>
              <SettingsRow label="Weekly digest email"><Switch bind:checked={notify.digest} label="Weekly digest email" /></SettingsRow>
              <SettingsRow label="When someone joins the workspace"><Switch bind:checked={notify.invites} label="When someone joins the workspace" /></SettingsRow>
            </Stack>
          </Section>
        {/if}
      </Reveal>
    {/key}
  </Split>
</Page>

<Dialog bind:open={deleting} title="Delete this workspace?" size="sm">
  <Text as="p" tone="secondary">All chats, files and tasks in {wsName} will be deleted for everyone.</Text>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="danger" onclick={() => { close(); toast('Deletion scheduled', { tone: 'danger' }); }}>Delete workspace</Button>
  {/snippet}
</Dialog>
