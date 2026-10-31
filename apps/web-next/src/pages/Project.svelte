<script>
  import { Star, Ellipsis, Pencil, Plus, UserPlus, Archive as ArchiveIcon, Trash2, MessagesSquare, FolderX } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Breadcrumbs from '../components/Breadcrumbs.svelte';
  import Stack from '../components/Stack.svelte';
  import Split from '../components/Split.svelte';
  import Card from '../components/Card.svelte';
  import Section from '../components/Section.svelte';
  import Composer from '../components/Composer.svelte';
  import List from '../components/List.svelte';
  import ListItem from '../components/ListItem.svelte';
  import IconButton from '../components/IconButton.svelte';
  import Menu from '../components/Menu.svelte';
  import MenuItem from '../components/MenuItem.svelte';
  import MenuSeparator from '../components/MenuSeparator.svelte';
  import Text from '../components/Text.svelte';
  import Icon from '../components/Icon.svelte';
  import Avatar from '../components/Avatar.svelte';
  import Attachment from '../components/Attachment.svelte';
  import Dialog from '../components/Dialog.svelte';
  import Textarea from '../components/Textarea.svelte';
  import Button from '../components/Button.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import InviteDialog from '../components/InviteDialog.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { app, ws, newChat, projectById, removeProject, addProjectFile } from '../lib/app.svelte.js';
  import { prompt, confirmDialog } from '../lib/dialogs.svelte.js';
  import { go } from '../lib/router.svelte.js';
  import { toast } from '../lib/toast.svelte.js';

  let { id } = $props();

  const project = $derived(projectById(id));
  let draft = $state('');
  let editing = $state(false);
  let inviting = $state(false);
  let value = $state('');

  const chats = $derived(ws().chats.filter((c) => c.project === id && !c.hidden));
  const files = $derived((project?.fileIds ?? []).map((fid) => ws().library.find((f) => f.id === fid)).filter(Boolean));

  function send({ text, attachments }) {
    const cid = newChat(text ? (text.length > 48 ? text.slice(0, 46) + '…' : text) : 'New chat', { project: id, text, attachments });
    go(`/chat/${cid}`);
  }

  async function rename() {
    const name = await prompt({ title: 'Rename project', label: 'Name', value: project.name });
    if (name) project.name = name;
  }

  async function remove() {
    if (await confirmDialog({ title: 'Delete this project?', body: 'Chats in it stay in your history.' })) removeProject(id);
  }
</script>

<TopBar>
  {#snippet leading()}
    <MobileMenu />
    <Breadcrumbs items={[{ label: 'Projects', href: '#/projects' }, { label: project?.name ?? 'Not found' }]} />
  {/snippet}
</TopBar>

{#if project}
  <Page width="wide">
    <PageHeader title={project.name}>
      {#snippet actions()}
        <IconButton icon={Star} label={project.starred ? 'Unstar' : 'Star'} pressed={project.starred} onclick={() => (project.starred = !project.starred)} />
        <Menu placement="bottom-end" label="Project actions">
          {#snippet trigger(props)}<IconButton icon={Ellipsis} label="More" {...props} />{/snippet}
          <MenuItem icon={Pencil} onselect={rename}>Rename</MenuItem>
          <MenuItem icon={UserPlus} onselect={() => (inviting = true)}>Invite people</MenuItem>
          <MenuItem icon={ArchiveIcon} onselect={() => removeProject(id, 'Project archived')}>Archive</MenuItem>
          <MenuSeparator />
          <MenuItem icon={Trash2} danger onselect={remove}>Delete</MenuItem>
        </Menu>
      {/snippet}
    </PageHeader>

    <Split size="lg">
      <Composer bind:value bind:model={app.model} size="lg" placeholder={`Start a chat in ${project.name}`} onsend={send} />
      <Section title="Chats">
        {#if chats.length}
          <List label="Project chats">
            {#each chats as chat (chat.id)}
              <ListItem href={`#/chat/${chat.id}`} title={chat.title}>
                {#snippet meta()}Last message {chat.updated.toLowerCase()}{/snippet}
              </ListItem>
            {/each}
          </List>
        {:else}
          <EmptyState icon={MessagesSquare} title="No chats in this project yet" size="sm" />
        {/if}
      </Section>

      {#snippet aside()}
        <Card padding="md" variant="filled">
          <Section title="Instructions">
            {#snippet actions()}<IconButton icon={Pencil} label="Edit instructions" size="sm" onclick={() => { draft = project.instructions; editing = true; }} />{/snippet}
            <Text as="p" size="sm" tone="secondary">{project.instructions || 'No instructions yet'}</Text>
          </Section>
        </Card>
        <Card padding="md" variant="filled">
          <Section title="Files">
            {#snippet actions()}<IconButton icon={Plus} label="Add files" size="sm" onclick={() => addProjectFile(id)} />{/snippet}
            <Stack gap="2">
              {#each files as f, i (f.id)}
                <Attachment name={f.name} kind={f.kind} size={f.size} tone={(i % 4) + 1} href={`#/library/${f.id}`} />
              {/each}
            </Stack>
            {#if !files.length}<Text size="sm" tone="tertiary">No files yet</Text>{/if}
          </Section>
        </Card>
        <Card padding="md" variant="filled">
          <Section title="People">
            {#snippet actions()}<IconButton icon={UserPlus} label="Invite" size="sm" onclick={() => (inviting = true)} />{/snippet}
            <Stack gap="3">
              {#each project.members as name (name)}
                <Stack direction="row" gap="2" align="center">
                  <Avatar {name} size="sm" />
                  <Text size="sm">{name}</Text>
                </Stack>
              {/each}
            </Stack>
          </Section>
        </Card>
      {/snippet}
    </Split>
  </Page>
{:else}
  <Page center>
    <EmptyState icon={FolderX} title="Project not found">
      <Button href="#/projects">All projects</Button>
    </EmptyState>
  </Page>
{/if}

<Dialog bind:open={editing} title="Project instructions">
  <Textarea bind:value={draft} rows={6} aria-label="Instructions" />
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" onclick={() => { project.instructions = draft; close(); toast('Instructions saved', { tone: 'success' }); }}>Save</Button>
  {/snippet}
</Dialog>

<InviteDialog bind:open={inviting} project={id} />
