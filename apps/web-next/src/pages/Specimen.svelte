<script>
  import {
    Plus, ArrowRight, Download, Trash2, Settings, Pin, Share, Copy, Pencil, Ellipsis, Search, Mail, Bold, Italic, Underline,
    LayoutGrid, List as ListIcon, Sun, Moon, Monitor, FolderOpen, FileText, MessagesSquare, Inbox, Bell, Info, Keyboard, Library, Lock
  } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Split from '../components/Split.svelte';
  import VerticalNav from '../components/VerticalNav.svelte';
  import SpecimenSection from '../components/SpecimenSection.svelte';
  import SpecimenBlock from '../components/SpecimenBlock.svelte';
  import Swatch from '../components/Swatch.svelte';
  import TypeSample from '../components/TypeSample.svelte';
  import SpaceSample from '../components/SpaceSample.svelte';
  import MotionSample from '../components/MotionSample.svelte';
  import Stack from '../components/Stack.svelte';
  import Grid from '../components/Grid.svelte';
  import Button from '../components/Button.svelte';
  import IconButton from '../components/IconButton.svelte';
  import Input from '../components/Input.svelte';
  import Textarea from '../components/Textarea.svelte';
  import Select from '../components/Select.svelte';
  import Checkbox from '../components/Checkbox.svelte';
  import Switch from '../components/Switch.svelte';
  import RadioGroup from '../components/RadioGroup.svelte';
  import Field from '../components/Field.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Tabs from '../components/Tabs.svelte';
  import SegmentedControl from '../components/SegmentedControl.svelte';
  import Breadcrumbs from '../components/Breadcrumbs.svelte';
  import Menu from '../components/Menu.svelte';
  import MenuItem from '../components/MenuItem.svelte';
  import MenuLabel from '../components/MenuLabel.svelte';
  import MenuSeparator from '../components/MenuSeparator.svelte';
  import SubMenu from '../components/SubMenu.svelte';
  import Popover from '../components/Popover.svelte';
  import Tooltip from '../components/Tooltip.svelte';
  import Dialog from '../components/Dialog.svelte';
  import Avatar from '../components/Avatar.svelte';
  import AvatarGroup from '../components/AvatarGroup.svelte';
  import Badge from '../components/Badge.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';
  import Chip from '../components/Chip.svelte';
  import Card from '../components/Card.svelte';
  import List from '../components/List.svelte';
  import ListItem from '../components/ListItem.svelte';
  import Table from '../components/Table.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import Skeleton from '../components/Skeleton.svelte';
  import Spinner from '../components/Spinner.svelte';
  import Kbd from '../components/Kbd.svelte';
  import Divider from '../components/Divider.svelte';
  import ProgressBar from '../components/ProgressBar.svelte';
  import FileTile from '../components/FileTile.svelte';
  import TypeList from '../components/TypeList.svelte';
  import Art from '../components/Art.svelte';
  import ServerSettings from '../components/ServerSettings.svelte';
  import TypeArt from '../components/TypeArt.svelte';
  import TypeDetail from '../components/TypeDetail.svelte';
  import { schemaFor } from '../lib/schema.js';
  import Attachment from '../components/Attachment.svelte';
  import Composer from '../components/Composer.svelte';
  import Message from '../components/Message.svelte';
  import Steps from '../components/Steps.svelte';
  import CodeBlock from '../components/CodeBlock.svelte';
  import StepPill from '../components/StepPill.svelte';
  import WorkflowCard from '../components/WorkflowCard.svelte';
  import VaultIcon from '../components/VaultIcon.svelte';
  import LookPicker from '../components/LookPicker.svelte';
  import ConnectorCard from '../components/ConnectorCard.svelte';
  import ConnectorIcon from '../components/ConnectorIcon.svelte';
  import TriggerLabel from '../components/TriggerLabel.svelte';
  import { workflowSeed } from '../lib/workflows.js';
  import { TYPES } from '../lib/looks.js';
  import { catalog } from '../lib/connectors.js';
  import Heading from '../components/Heading.svelte';
  import Text from '../components/Text.svelte';
  import Sidebar from '../components/Sidebar.svelte';
  import VaultRail from '../components/VaultRail.svelte';
  import SidebarItem from '../components/SidebarItem.svelte';
  import SidebarSection from '../components/SidebarSection.svelte';
  import VaultHeader from '../components/VaultHeader.svelte';
  import ThemePicker from '../components/ThemePicker.svelte';
  import Disclosure from '../components/Disclosure.svelte';
  import SettingsRow from '../components/SettingsRow.svelte';
  import Greeting from '../components/Greeting.svelte';
  import SpecimenFrame from '../components/SpecimenFrame.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import SettingsNav from '../components/SettingsNav.svelte';
  import SettingsPane from '../components/SettingsPane.svelte';
  import NotificationSettings from '../components/NotificationSettings.svelte';
  import ConnectorDetail from '../components/ConnectorDetail.svelte';
  import { openSettings } from '../lib/app.svelte.js';
  import { User as UserIcon, Palette as PaletteIcon, Bell as BellIcon, Plug as PlugIcon } from '@lucide/svelte';
  import { library, conversations, members, me } from '../lib/data.js';
  import { theme, setTheme } from '../lib/theme.svelte.js';
  import { toast } from '../lib/toast.svelte.js';
  import { app } from '../lib/app.svelte.js';
  const familySchema = schemaFor('okafor');

  const sections = [
    ['color', 'Color'], ['type', 'Typography'], ['space', 'Space and shape'], ['motion', 'Motion'],
    ['buttons', 'Buttons'], ['inputs', 'Inputs'], ['selection', 'Selection'], ['navigation', 'Navigation'],
    ['overlays', 'Overlays'], ['display', 'Data display'], ['feedback', 'Feedback'], ['chat', 'Chat'], ['shell', 'Shell']
  ];
  let current = $state('color');
  const nav = sections.map(([id, label]) => ({ id, label, onclick: () => { current = id; document.getElementById(`s-${id}`)?.scrollIntoView({ behavior: 'smooth' }); } }));

  const colors = ['canvas', 'sidebar', 'surface', 'surface-raised', 'surface-sunken', 'bubble', 'code', 'fill-subtle', 'fill', 'fill-strong', 'border', 'border-strong'];
  const ink = ['text', 'text-secondary', 'text-tertiary', 'text-disabled', 'inverse', 'primary', 'accent', 'accent-subtle', 'success', 'warning', 'danger'];
  const avatars = [1, 2, 3, 4, 5, 6];
  const sizes = ['text-4xl', 'text-3xl', 'text-2xl', 'text-xl', 'text-lg', 'text-base', 'text-md', 'text-sm', 'text-xs', 'text-2xs'];
  const spaces = ['space-1', 'space-2', 'space-3', 'space-4', 'space-5', 'space-6', 'space-8', 'space-10', 'space-12', 'space-16', 'space-20', 'space-24'];
  const radii = ['radius-xs', 'radius-sm', 'radius-md', 'radius-lg', 'radius-xl', 'radius-2xl', 'radius-full'];
  const shadows = ['ring-subtle', 'ring', 'ring-strong'];

  let text = $state('Leeds, winter 1974');
  let note = $state('');
  let select = $state('editor');
  let checks = $state({ a: true, b: false, c: false });
  let toggles = $state({ a: true, b: false });
  let radio = $state('month');
  let tab = $state('overview');
  let pills = $state('all');
  let seg = $state('grid');
  let seg2 = $state('system');
  let dialog = $state(false);
  let confirm = $state(false);
  let composer = $state('');
  let streaming = $state(false);
  let disclosure = $state(true);
  let look = $state({ type: 'family', icon: 'castle', tint: 4 });
  let settingsSection = $state('appearance');

  const roles = [
    { value: 'viewer', label: 'Viewer' },
    { value: 'editor', label: 'Editor' },
    { value: 'owner', label: 'Owner' }
  ];
  const columns = [
    { key: 'name', label: 'Name' },
    { key: 'role', label: 'Role', width: 'sm' },
    { key: 'active', label: 'Last active', muted: true, width: 'sm', align: 'end' }
  ];
  const runningSteps = [
    { id: 'a', label: 'Reading the 214 scans in Family Photos', status: 'done', time: '4s' },
    { id: 'b', label: 'Pulling the date each photo was taken', status: 'running' },
    { id: 'c', label: 'Renaming 198 files and setting 16 aside without a date', status: 'pending' }
  ];
</script>

<TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <PageHeader title="Specimen" size="lg">
    {#snippet actions()}
      <SegmentedControl value={theme.choice} options={[{ value: 'light', label: 'Light', icon: Sun }, { value: 'dark', label: 'Dark', icon: Moon }, { value: 'system', label: 'System', icon: Monitor }]} label="Theme" onchange={setTheme} />
    {/snippet}
  </PageHeader>

  <Split side="start" sticky>
    {#snippet aside()}<VerticalNav items={nav} active={current} label="Sections" />{/snippet}

    <Stack gap="16">
      <SpecimenSection id="s-color" title="Color">
        <SpecimenBlock title="Surfaces and fills">
          <Grid min="tile" gap="4">{#each colors as c (c)}<Swatch token={`color-${c}`} label={c} />{/each}</Grid>
        </SpecimenBlock>
        <SpecimenBlock title="Ink, accent and status">
          <Grid min="tile" gap="4">{#each ink as c (c)}<Swatch token={`color-${c}`} label={c} />{/each}</Grid>
        </SpecimenBlock>
        <SpecimenBlock title="Avatar tones">
          <Grid min="tile" gap="4">{#each avatars as a (a)}<Swatch token={`color-avatar-${a}`} label={`avatar-${a}`} />{/each}</Grid>
        </SpecimenBlock>
      </SpecimenSection>

      <SpecimenSection id="s-type" title="Typography">
        <SpecimenBlock title="Families" column>
          <TypeSample token="text-2xl" family="serif" label="EB Garamond">Letters from Enugu, 1974</TypeSample>
          <TypeSample token="text-xl" family="sans" label="Inter">Letters from Enugu, 1974</TypeSample>
        </SpecimenBlock>
        <SpecimenBlock title="Serif scale" column>
          {#each sizes.slice(0, 5) as s (s)}<TypeSample token={s} family="serif">The archive remembers</TypeSample>{/each}
        </SpecimenBlock>
        <SpecimenBlock title="Sans scale" column>
          {#each sizes as s (s)}<TypeSample token={s}>The archive remembers everything you add</TypeSample>{/each}
        </SpecimenBlock>
        <SpecimenBlock title="Headings and text" column>
          <Heading level={3} size="2xl">Display heading</Heading>
          <Heading level={3} size="xl">Page heading</Heading>
          <Heading level={3} size="lg">Dialog heading</Heading>
          <Heading level={3} size="md" serif={false}>Section heading</Heading>
          <Stack direction="row" gap="4" wrap>
            <Text>Default</Text><Text tone="secondary">Secondary</Text><Text tone="tertiary">Tertiary</Text><Text tone="accent">Accent</Text>
            <Text tone="success">Success</Text><Text tone="warning">Warning</Text><Text tone="danger">Danger</Text>
          </Stack>
        </SpecimenBlock>
      </SpecimenSection>

      <SpecimenSection id="s-space" title="Space and shape">
        <SpecimenBlock title="Spacing scale" column>
          {#each spaces as s (s)}<SpaceSample token={s} />{/each}
        </SpecimenBlock>
        <SpecimenBlock title="Radii">
          <Grid min="tile" gap="4">{#each radii as r (r)}<Swatch token={r} />{/each}</Grid>
        </SpecimenBlock>
        <SpecimenBlock title="Rings">
          <Grid min="tile" gap="4">{#each shadows as s (s)}<Swatch token={s} />{/each}</Grid>
        </SpecimenBlock>
      </SpecimenSection>

      <SpecimenSection id="s-motion" title="Motion">
        <SpecimenBlock title="Durations, click to play" column>
          <MotionSample duration="fast" easing="out" label="fast, ease-out" />
          <MotionSample duration="normal" easing="out" label="normal, ease-out" />
          <MotionSample duration="slow" easing="in-out" label="slow, ease-in-out" />
          <MotionSample duration="slower" easing="emphasized" label="slower, emphasized" />
        </SpecimenBlock>
        <SpecimenBlock title="Vault icons, hover to wake">
          {#each Object.entries(TYPES) as [type, t] (type)}
            {#each t.icons as icon, i (icon)}
              <VaultIcon {icon} tint={i % 8} seed={type} size="xl" label={`${t.label}: ${icon}`} />
            {/each}
          {/each}
        </SpecimenBlock>
        <SpecimenBlock title="Vault icon sizes">
          <VaultIcon icon="cottage" tint={0} size="xs" /><VaultIcon icon="cottage" tint={0} size="sm" /><VaultIcon icon="cottage" tint={0} /><VaultIcon icon="cottage" tint={0} size="lg" /><VaultIcon icon="cottage" tint={0} size="xl" /><VaultIcon icon="robot" tint={3} size="xxl" />
        </SpecimenBlock>
        <SpecimenBlock title="Look picker" column>
          <LookPicker bind:look />
        </SpecimenBlock>
        <SpecimenBlock title="Working indicator">
          <VaultIcon icon="cottage" tint={0} size="sm" active label="Working" />
        </SpecimenBlock>
      </SpecimenSection>

      <SpecimenSection id="s-buttons" title="Buttons">
        <SpecimenBlock title="Variants">
          <Button variant="primary">Primary</Button>
          <Button variant="inverse">Inverse</Button>
          <Button>Secondary</Button>
          <Button variant="ghost">Ghost</Button>
          <Button variant="subtle">Subtle</Button>
          <Button variant="accent">Accent</Button>
          <Button variant="danger">Danger</Button>
          <Button variant="link">Link</Button>
        </SpecimenBlock>
        <SpecimenBlock title="Sizes">
          <Button variant="primary" size="sm">Small</Button>
          <Button variant="primary">Medium</Button>
          <Button variant="primary" size="lg">Large</Button>
          <Button size="sm">Small</Button>
          <Button>Medium</Button>
          <Button size="lg">Large</Button>
        </SpecimenBlock>
        <SpecimenBlock title="With icons and states">
          <Button variant="primary" icon={Plus}>New chat</Button>
          <Button icon={Download}>Download</Button>
          <Button iconEnd={ArrowRight}>Continue</Button>
          <Button variant="danger" icon={Trash2}>Delete</Button>
          <Button variant="primary" loading>Saving</Button>
          <Button disabled>Disabled</Button>
          <Button variant="primary" disabled>Disabled</Button>
        </SpecimenBlock>
        <SpecimenBlock title="Icon buttons">
          <IconButton icon={Settings} label="Ghost" />
          <IconButton icon={Pin} label="Pressed" pressed />
          <IconButton icon={Share} label="Subtle" variant="subtle" />
          <IconButton icon={Copy} label="Secondary" variant="secondary" />
          <IconButton icon={ArrowRight} label="Accent round" variant="accent" round />
          <IconButton icon={Plus} label="Primary round" variant="primary" round />
          <IconButton icon={Plus} label="Inverse round" variant="inverse" round />
          <IconButton icon={Pencil} label="Extra small" size="xs" />
          <IconButton icon={Pencil} label="Small" size="sm" />
          <IconButton icon={Pencil} label="Large" size="lg" />
          <IconButton icon={Trash2} label="Disabled" disabled />
        </SpecimenBlock>
      </SpecimenSection>

      <SpecimenSection id="s-inputs" title="Inputs">
        <SpecimenBlock title="Text fields" column>
          <Grid columns="2" gap="4">
            <Field label="Default">{#snippet children(id)}<Input {id} bind:value={text} />{/snippet}</Field>
            <Field label="With icon">{#snippet children(id)}<Input {id} icon={Mail} placeholder="name@example.com" />{/snippet}</Field>
            <Field label="Small">{#snippet children(id)}<Input {id} size="sm" placeholder="Small input" />{/snippet}</Field>
            <Field label="Large">{#snippet children(id)}<Input {id} size="lg" placeholder="Large input" />{/snippet}</Field>
            <Field label="Invalid" error="That email is already a member">{#snippet children(id)}<Input {id} invalid value="ruth@hale.law" />{/snippet}</Field>
            <Field label="Disabled">{#snippet children(id)}<Input {id} disabled value="Read only" />{/snippet}</Field>
          </Grid>
        </SpecimenBlock>
        <SpecimenBlock title="Search" column>
          <SearchField placeholder="Search files" shortcut="/" />
          <SearchField placeholder="Search with value" value="letters" size="lg" />
        </SpecimenBlock>
        <SpecimenBlock title="Textarea" column>
          <Field label="Notes">{#snippet children(id)}<Textarea {id} bind:value={note} placeholder="Write something" />{/snippet}</Field>
          <Field label="Autosize">{#snippet children(id)}<Textarea {id} autosize rows={1} placeholder="Grows as you type" />{/snippet}</Field>
        </SpecimenBlock>
        <SpecimenBlock title="Select">
          <Select bind:value={select} options={roles} label="Role" />
          <Select bind:value={select} options={roles} size="sm" label="Role" />
          <Select bind:value={select} options={roles} variant="ghost" label="Role" />
          <Select options={roles} placeholder="Pick a role" label="Role" />
          <Select options={roles} disabled placeholder="Disabled" label="Role" />
        </SpecimenBlock>
      </SpecimenSection>

      <SpecimenSection id="s-selection" title="Selection">
        <SpecimenBlock title="Checkbox">
          <Checkbox bind:checked={checks.a}>Checked</Checkbox>
          <Checkbox bind:checked={checks.b}>Unchecked</Checkbox>
          <Checkbox indeterminate>Indeterminate</Checkbox>
          <Checkbox disabled>Disabled</Checkbox>
        </SpecimenBlock>
        <SpecimenBlock title="Switch">
          <Switch bind:checked={toggles.a} label="On" />
          <Switch bind:checked={toggles.b} label="Off" />
          <Switch disabled label="Disabled" />
          <Switch checked disabled label="Disabled on" />
        </SpecimenBlock>
        <SpecimenBlock title="Radio">
          <RadioGroup bind:value={radio} label="Billing" direction="row" options={[{ value: 'month', label: 'Monthly' }, { value: 'year', label: 'Yearly' }, { value: 'life', label: 'Lifetime', disabled: true }]} />
        </SpecimenBlock>
        <SpecimenBlock title="Chips">
          <Chip>Static</Chip>
          <Chip icon={FolderOpen} onclick={() => {}}>Interactive</Chip>
          <Chip selected onclick={() => {}}>Selected</Chip>
          <Chip onremove={() => toast('Removed')}>Removable</Chip>
          <Chip size="sm">Small</Chip>
          <Chip size="sm" onremove={() => {}}>Small removable</Chip>
        </SpecimenBlock>
        <SpecimenBlock title="Theme picker">
          <ThemePicker value={theme.choice} onchange={setTheme} />
        </SpecimenBlock>
      </SpecimenSection>

      <SpecimenSection id="s-navigation" title="Navigation">
        <SpecimenBlock title="Tabs" column>
          <Tabs bind:value={tab} label="Line tabs" items={[{ value: 'overview', label: 'Overview' }, { value: 'files', label: 'Files', count: 86 }, { value: 'people', label: 'People', count: 9 }]} />
          <Tabs bind:value={pills} variant="pill" label="Pill tabs" items={[{ value: 'all', label: 'All' }, { value: 'mine', label: 'Mine' }, { value: 'shared', label: 'Shared' }]} />
        </SpecimenBlock>
        <SpecimenBlock title="Segmented control">
          <SegmentedControl bind:value={seg} label="View" options={[{ value: 'grid', label: 'Grid', icon: LayoutGrid }, { value: 'list', label: 'List', icon: ListIcon }]} />
          <SegmentedControl bind:value={seg} label="View" iconOnly options={[{ value: 'grid', label: 'Grid', icon: LayoutGrid }, { value: 'list', label: 'List', icon: ListIcon }]} />
          <SegmentedControl bind:value={seg2} size="sm" label="Theme" options={[{ value: 'light', label: 'Light' }, { value: 'dark', label: 'Dark' }, { value: 'system', label: 'System' }]} />
          <SegmentedControl bind:value={seg2} size="lg" label="Theme" options={[{ value: 'light', label: 'Light' }, { value: 'dark', label: 'Dark' }, { value: 'system', label: 'System' }]} />
        </SpecimenBlock>
        <SpecimenBlock title="Breadcrumbs">
          <Breadcrumbs items={[{ label: 'Library', href: '#/library' }, { label: '1970s Letters', href: '#/library' }, { label: 'Adaeze to Obi, 2 Mar 1974' }]} />
        </SpecimenBlock>
        <SpecimenBlock title="Vertical nav">
          <VerticalNav label="Example" active="b" items={[{ id: 'a', label: 'Profile', href: '#/specimen' }, { id: 'b', label: 'Appearance', href: '#/specimen' }, { id: 'c', label: 'Vault', href: '#/specimen' }]} />
        </SpecimenBlock>
        <SpecimenBlock title="Keyboard">
          <Kbd keys="⌘ K" /><Kbd keys="⇧ ⌘ O" /><Kbd keys="Esc" /><Kbd keys="↵" /><Kbd keys="⌘ ," subtle />
        </SpecimenBlock>
      </SpecimenSection>

      <SpecimenSection id="s-overlays" title="Overlays">
        <SpecimenBlock title="Menu with submenu, checks, shortcuts and danger">
          <Menu label="Example menu">
            {#snippet trigger(props)}<Button iconEnd={Ellipsis} {...props}>Open menu</Button>{/snippet}
            <MenuLabel>nora@okafor.family</MenuLabel>
            <MenuItem icon={Settings} shortcut="⌘ ,">Settings</MenuItem>
            <MenuItem icon={Pin} checked>Pinned</MenuItem>
            <MenuItem icon={Bell} keepOpen toggle={toggles.a} onselect={() => (toggles.a = !toggles.a)}>Notifications</MenuItem>
            <SubMenu icon={FolderOpen} label="Move to folder">
              <MenuItem>1970s Letters</MenuItem>
              <MenuItem>Photo albums</MenuItem>
              <MenuItem>Family tree</MenuItem>
            </SubMenu>
            <MenuItem icon={Copy} disabled>Duplicate</MenuItem>
            <MenuSeparator />
            <MenuItem icon={Trash2} danger>Delete</MenuItem>
          </Menu>
          <Menu label="Placement" placement="bottom-end">
            {#snippet trigger(props)}<IconButton icon={Ellipsis} label="End-aligned menu" {...props} />{/snippet}
            <MenuItem icon={Pencil}>Rename</MenuItem>
            <MenuItem icon={Share}>Share</MenuItem>
          </Menu>
        </SpecimenBlock>
        <SpecimenBlock title="Popover and tooltip">
          <Popover label="Share">
            {#snippet trigger(props)}<Button icon={Share} {...props}>Share</Button>{/snippet}
            {#snippet children({ close })}
              <Stack gap="3">
                <Text weight="medium">Share this chat</Text>
                <Input size="sm" value="https://app.archive.dev/s/8f2k" readonly aria-label="Link" />
                <Stack direction="row" gap="2" justify="end">
                  <Button size="sm" variant="ghost" onclick={close}>Close</Button>
                  <Button size="sm" variant="primary" icon={Copy} onclick={() => { close(); toast('Link copied'); }}>Copy link</Button>
                </Stack>
              </Stack>
            {/snippet}
          </Popover>
          <Tooltip text="Tooltips appear after a short delay"><Button variant="ghost" icon={Info}>Hover me</Button></Tooltip>
          <Tooltip text="With a shortcut" shortcut="⌘ K" placement="bottom"><Button variant="ghost" icon={Keyboard}>And me</Button></Tooltip>
        </SpecimenBlock>
        <SpecimenBlock title="Settings dialog">
          <Button onclick={() => openSettings()}>Open settings</Button>
          <Button onclick={() => openSettings('connectors')}>Open connectors</Button>
          <Button onclick={() => openSettings('connectors', 'gdrive')}>Open a connector</Button>
        </SpecimenBlock>
        <SpecimenBlock title="Settings nav">
          <SettingsNav active={settingsSection} onselect={(id) => (settingsSection = id)} items={[{ id: 'profile', label: 'Profile', icon: UserIcon }, { id: 'appearance', label: 'Appearance', icon: PaletteIcon }, { id: 'notifications', label: 'Notifications', icon: BellIcon }, { id: 'connectors', label: 'Connectors', icon: PlugIcon, group: 'Customize' }, { id: 'api', label: 'API keys', icon: PlugIcon, group: 'Platform', href: '#/specimen' }]} />
        </SpecimenBlock>
        <SpecimenBlock title="Settings pane" column>
          <Card padding="none"><SettingsPane title="Notifications"><NotificationSettings /></SettingsPane></Card>
          <Card padding="none"><SettingsPane title={catalog[0].name} onback={() => toast('Back pressed')}><ConnectorDetail connector={{ ...catalog[0], connected: true, account: 'nora.okafor@gmail.com', lastSync: '2 min ago', items: 412 }} /></SettingsPane></Card>
        </SpecimenBlock>
        <SpecimenBlock title="Dialog, toast and command palette">
          <Button onclick={() => (dialog = true)}>Open dialog</Button>
          <Button variant="danger" onclick={() => (confirm = true)}>Confirm dialog</Button>
          <Button onclick={() => toast('Saved to 1970s Letters')}>Neutral toast</Button>
          <Button onclick={() => toast('Invitation sent', { tone: 'success' })}>Success toast</Button>
          <Button onclick={() => toast('Upload failed', { tone: 'danger', action: { label: 'Retry', run: () => {} } })}>Danger toast</Button>
          <Button icon={Search} onclick={() => (app.palette = true)}>Command palette</Button>
        </SpecimenBlock>
      </SpecimenSection>

      <SpecimenSection id="s-display" title="Data display">
        <SpecimenBlock title="Avatars">
          <Avatar name="Nora Okafor" size="xs" />
          <Avatar name="Nora Okafor" size="sm" />
          <Avatar name="Chidi Okafor" />
          <Avatar name="Ruth Hale" size="lg" status="online" />
          <Avatar name="Jun Park" size="xl" />
          <Avatar name="Okafor Family Archive" square tone={1} />
          <Avatar name="Fieldnote Studio" square size="lg" tone={2} />
        </SpecimenBlock>
        <SpecimenBlock title="Avatar groups">
          <AvatarGroup names={members.slice(0, 3).map((m) => m.name)} size="xs" />
          <AvatarGroup names={members.map((m) => m.name)} />
          <AvatarGroup names={members.map((m) => m.name)} size="md" max={5} />
        </SpecimenBlock>
        <SpecimenBlock title="Badges">
          <Badge>Plain</Badge><Badge icon={Pin}>With icon</Badge>
          {#each ['running', 'waiting', 'succeeded', 'failed', 'stopped', 'paused', 'scheduled', 'off', 'idle', 'invited', 'connected'] as st (st)}<StatusBadge status={st} />{/each}
        </SpecimenBlock>
        <SpecimenBlock title="Cards">
          <Grid min="card" gap="4">
            <Card><Text weight="medium">Outline</Text><Text size="sm" tone="secondary">Default card surface.</Text></Card>
            <Card variant="filled"><Text weight="medium">Filled</Text><Text size="sm" tone="secondary">For asides and groups.</Text></Card>
            <Card variant="raised"><Text weight="medium">Raised</Text><Text size="sm" tone="secondary">For floating content.</Text></Card>
            <Card href="#/specimen"><Text weight="medium">Interactive</Text><Text size="sm" tone="secondary">Hover and press states.</Text></Card>
          </Grid>
        </SpecimenBlock>
        <SpecimenBlock title="List" column>
          <List label="Example list">
            <ListItem href="#/specimen" title="Transcribe Grandma Adaeze's 1974 letters">
              {#snippet leading()}<MessagesSquare size={16} />{/snippet}
              {#snippet meta()}Last message 12 min ago{/snippet}
              {#snippet trailing()}<Badge>Pinned</Badge>{/snippet}
            </ListItem>
            <ListItem href="#/specimen" title="Timeline for the Mercer lease dispute" active>
              {#snippet leading()}<MessagesSquare size={16} />{/snippet}
              {#snippet meta()}Active row{/snippet}
            </ListItem>
            <ListItem title="Static row" size="sm">
              {#snippet leading()}<FileText size={16} />{/snippet}
              {#snippet trailing()}2.4 MB{/snippet}
            </ListItem>
          </List>
        </SpecimenBlock>
        <SpecimenBlock title="Table" column>
          <Table {columns} rows={members.slice(0, 4)} label="Members" onrowclick={(r) => toast(r.name)}>
            {#snippet cell(row, col)}
              {#if col.key === 'name'}
                <Stack direction="row" gap="2" align="center"><Avatar name={row.name} size="sm" /><Text weight="medium">{row.name}</Text></Stack>
              {:else if col.key === 'role'}
                <Chip size="sm">{row.role}</Chip>
              {:else}{row[col.key]}{/if}
            {/snippet}
          </Table>
        </SpecimenBlock>
        <SpecimenBlock title="File tiles">
          <Grid min="tile" gap="2">
            {#each [library[0], library[1], library[2], library[4], library[5]] as f (f.id)}<FileTile file={f} />{/each}
          </Grid>
        </SpecimenBlock>
        <SpecimenBlock title="Type list" column>
          <TypeList types={familySchema.slice(0, 4)} all={familySchema} />
        </SpecimenBlock>
        <SpecimenBlock title="Type art">
          {#each familySchema as t (t.id)}<TypeArt icon={t.icon} size="sm" />{/each}
        </SpecimenBlock>
        <SpecimenBlock title="Art">
          <Art name="server" />
        </SpecimenBlock>
        <SpecimenBlock title="Server settings" column>
          <ServerSettings />
        </SpecimenBlock>
        <SpecimenBlock title="Type detail" column>
          <TypeDetail type={familySchema[0]} types={familySchema} />
        </SpecimenBlock>
        <SpecimenBlock title="Attachments">
          <Attachment name="adaeze-letter-03.jpg" kind="image" size="2.4 MB" tone={4} />
          <Attachment name="Linden kickoff notes.pdf" kind="pdf" size="480 KB" tone={1} />
          <Attachment name="Cassette 4, side A.wav" kind="recording" size="96 MB" tone={2} onremove={() => {}} />
        </SpecimenBlock>
        <SpecimenBlock title="Progress, divider and disclosure" column>
          <ProgressBar value={0.25} />
          <ProgressBar value={0.6} tone="success" />
          <ProgressBar value={0.85} tone="danger" size="sm" />
          <Divider />
          <Divider label="or" />
          <Disclosure bind:open={disclosure}>
            {#snippet summary()}Show details{/snippet}
            <Text as="p" size="sm" tone="secondary">Disclosures animate open and closed with the slide transition.</Text>
          </Disclosure>
          <Stack gap="0">
            <SettingsRow label="Settings row"><Switch checked label="Example" /></SettingsRow>
            <SettingsRow label="Another row"><Select value="viewer" options={roles} size="sm" label="Role" /></SettingsRow>
          </Stack>
        </SpecimenBlock>
      </SpecimenSection>

      <SpecimenSection id="s-feedback" title="Feedback">
        <SpecimenBlock title="Empty states" column>
          <Grid columns="2" gap="4">
            <Card variant="filled" padding="none"><EmptyState icon={Inbox} title="Nothing here yet"><Button size="sm" variant="primary" icon={Plus}>Add files</Button></EmptyState></Card>
            <Card variant="filled" padding="none"><EmptyState icon={Search} title="No results" size="sm" /></Card>
          </Grid>
        </SpecimenBlock>
        <SpecimenBlock title="Skeletons" column>
          <Stack direction="row" gap="3" align="center"><Skeleton variant="circle" /><Skeleton lines={2} /></Stack>
          <Grid min="tile" gap="4"><Skeleton variant="block" /><Skeleton variant="block" /><Skeleton variant="block" /></Grid>
          <Skeleton variant="row" />
        </SpecimenBlock>
        <SpecimenBlock title="Spinners">
          <Spinner size="sm" /><Spinner /><Spinner size="lg" /><Spinner size="xl" />
        </SpecimenBlock>
      </SpecimenSection>

      <SpecimenSection id="s-chat" title="Chat">
        <SpecimenBlock title="Greeting" column>
          <Greeting name={me.first} />
        </SpecimenBlock>
        <SpecimenBlock title="Composer: home, chat, streaming" column>
          <Composer bind:value={composer} size="lg" />
          <Composer placeholder="Reply to Archive" />
          <Composer placeholder="Reply to Archive" streaming />
        </SpecimenBlock>
        <SpecimenBlock title="Messages" column>
          <Message message={conversations['letters-1974'][0]} />
          <Message message={conversations['letters-1974'][3]} />
        </SpecimenBlock>
        <SpecimenBlock title="Work disclosure: running and finished" column>
          <Steps steps={runningSteps} running open />
          <Steps steps={conversations['mercer-timeline'][1].steps} worked="25s" />
          <StepPill label="Transcribe cassette 7" done={3} total={6} running />
          <StepPill label="Email 9 members" done={4} total={4} />
        </SpecimenBlock>
        <SpecimenBlock title="Code block" column>
          <CodeBlock lang="bash" code={'python3 -m pip install pillow\npython3 rename_scans.py --dry-run'} />
        </SpecimenBlock>
        <SpecimenBlock title="Workflow cards and triggers">
          <Grid min="card" gap="4">
            <WorkflowCard workflow={workflowSeed.okafor[0]} href="#/workflows/wf-cassettes" />
            <WorkflowCard workflow={workflowSeed.okafor[2]} href="#/workflows/wf-digest" />
            <WorkflowCard workflow={workflowSeed.okafor[4]} href="#/workflows/wf-restore" />
          </Grid>
          <TriggerLabel trigger={{ kind: 'schedule', label: 'Every Sunday at 18:00' }} />
          <TriggerLabel trigger={{ kind: 'file', label: 'When photos are added' }} />
          <TriggerLabel trigger={{ kind: 'email', label: 'When an email arrives' }} />
          <TriggerLabel trigger={{ kind: 'manual', label: 'Run by hand' }} />
        </SpecimenBlock>
        <SpecimenBlock title="Connector icons">
          {#each catalog as c (c.id)}<ConnectorIcon id={c.id} label={c.name} />{/each}
        </SpecimenBlock>
        <SpecimenBlock title="Connector icon sizes">
          <ConnectorIcon id="github" size="xs" /><ConnectorIcon id="github" size="sm" /><ConnectorIcon id="github" /><ConnectorIcon id="github" size="lg" /><ConnectorIcon id="slack" size="lg" />
        </SpecimenBlock>
        <SpecimenBlock title="Connectors">
          <Grid columns="2" gap="3">
            <ConnectorCard connector={{ ...catalog[0], connected: true, account: 'nora.okafor@gmail.com' }} />
            <ConnectorCard connector={{ ...catalog[8], connected: false }} onconnect={() => toast('Connect pressed')} />
          </Grid>
        </SpecimenBlock>
      </SpecimenSection>

      <SpecimenSection id="s-shell" title="Shell">
        <SpecimenBlock title="Vault rail">
          <SpecimenFrame>
            <Sidebar collapsed>
              {#snippet rail()}<VaultRail vaults={app.vaults} current={app.vaults[0].id} onselect={(id) => toast(`Picked ${id}`)} oncreate={() => toast('New vault pressed')} />{/snippet}
              {#snippet header()}<VaultHeader vault={app.vaults[0]} />{/snippet}
              <SidebarSection>
                <SidebarItem icon={Plus} label="New chat" accent href="#/specimen" />
                <SidebarItem icon={Search} label="Search" />
                <SidebarItem icon={MessagesSquare} label="Chats" href="#/specimen" active />
              </SidebarSection>
            </Sidebar>
          </SpecimenFrame>
        </SpecimenBlock>
        <SpecimenBlock title="Sidebar, open and collapsed">
          <SpecimenFrame>
            <Sidebar>
              {#snippet header()}<VaultHeader vault={app.vaults[0]} />{/snippet}
              <SidebarSection>
                <SidebarItem icon={Plus} label="New chat" accent href="#/specimen" />
                <SidebarItem icon={Search} label="Search" shortcut="⌘ K" />
              </SidebarSection>
              <SidebarSection>
                <SidebarItem icon={MessagesSquare} label="Chats" href="#/specimen" active />
                <SidebarItem icon={Library} label="Library" href="#/specimen" />
                <SidebarItem icon={Lock} label="Access" href="#/specimen" badge="3" />
              </SidebarSection>
              <SidebarSection title="Today">
                <SidebarItem label="Rename scanned photos by EXIF date" href="#/specimen">
                  {#snippet actions()}<IconButton icon={Ellipsis} label="More" size="xs" tooltip={false} />{/snippet}
                </SidebarItem>
              </SidebarSection>
            </Sidebar>
          </SpecimenFrame>
          <SpecimenFrame>
            <Sidebar collapsed>
              {#snippet header()}<VaultHeader vault={app.vaults[1]} />{/snippet}
              <SidebarSection>
                <SidebarItem icon={Plus} label="New chat" accent href="#/specimen" />
                <SidebarItem icon={Search} label="Search" />
                <SidebarItem icon={MessagesSquare} label="Chats" href="#/specimen" active />
                <SidebarItem icon={Library} label="Library" href="#/specimen" />
              </SidebarSection>
            </Sidebar>
          </SpecimenFrame>
        </SpecimenBlock>
      </SpecimenSection>
    </Stack>
  </Split>
</Page>

<Dialog bind:open={dialog} title="Rename chat">
  <Field label="Title">{#snippet children(id)}<Input {id} value="Transcribe Grandma Adaeze's 1974 letters" />{/snippet}</Field>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" onclick={close}>Save</Button>
  {/snippet}
</Dialog>

<Dialog bind:open={confirm} title="Delete this chat?" size="sm">
  <Text as="p" tone="secondary">This can’t be undone.</Text>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="danger" onclick={() => { close(); toast('Chat deleted'); }}>Delete</Button>
  {/snippet}
</Dialog>
