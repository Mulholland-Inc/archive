<script>
  import { Ellipsis, ChevronDown, Star, StarOff, Pencil, FolderInput, Trash2, Share, FolderMinus } from '@lucide/svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import SubMenu from './SubMenu.svelte';
  import IconButton from './IconButton.svelte';
  import Button from './Button.svelte';
  import { ws, toggleStar, removeChat, renameChat, moveChat, copyLink } from '../lib/app.svelte.js';
  import { prompt, confirmDialog } from '../lib/dialogs.svelte.js';

  let { chat, placement = 'bottom-start', size = 'xs', title = false } = $props();

  async function rename() {
    const next = await prompt({ title: 'Rename chat', label: 'Title', value: chat.title });
    if (next) renameChat(chat.id, next);
  }

  async function remove() {
    if (await confirmDialog({ title: 'Delete this chat?', body: `“${chat.title}” will be removed.`, confirm: 'Delete' })) removeChat(chat.id);
  }
</script>

<Menu {placement} label="Chat actions" width="sm">
  {#snippet trigger(props)}
    {#if title}
      <Button variant="ghost" iconEnd={ChevronDown} {...props}>{chat.title}</Button>
    {:else}
      <IconButton icon={Ellipsis} label={`Actions for ${chat.title}`} {size} tooltip={false} {...props} />
    {/if}
  {/snippet}
  <MenuItem icon={chat.starred ? StarOff : Star} onselect={() => toggleStar(chat.id)}>{chat.starred ? 'Unstar' : 'Star'}</MenuItem>
  <MenuItem icon={Pencil} onselect={rename}>Rename</MenuItem>
  {#if ws().projects.length}
    <SubMenu icon={FolderInput} label={chat.project ? 'Move to project' : 'Add to project'}>
      {#each ws().projects as p (p.id)}
        <MenuItem checked={chat.project === p.id} onselect={() => moveChat(chat.id, p.id)}>{p.name}</MenuItem>
      {/each}
    </SubMenu>
  {/if}
  {#if chat.project}
    <MenuItem icon={FolderMinus} onselect={() => moveChat(chat.id, chat.project)}>Remove from project</MenuItem>
  {/if}
  <MenuItem icon={Share} onselect={() => copyLink(`/chat/${chat.id}`)}>Copy link</MenuItem>
  <MenuSeparator />
  <MenuItem icon={Trash2} danger onselect={remove}>Delete</MenuItem>
</Menu>
