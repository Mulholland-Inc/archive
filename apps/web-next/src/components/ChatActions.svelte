<script>
  import { Ellipsis, ChevronDown, Star, StarOff, Pencil, FolderInput, Trash2, Share } from '@lucide/svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import SubMenu from './SubMenu.svelte';
  import IconButton from './IconButton.svelte';
  import Button from './Button.svelte';
  import { projects } from '../lib/data.js';
  import { toggleStar, removeChat, restoreChat } from '../lib/app.svelte.js';
  import { toast } from '../lib/toast.svelte.js';
  import { go, route } from '../lib/router.svelte.js';

  let { chat, placement = 'bottom-start', size = 'xs', title = false } = $props();

  function remove() {
    const removed = removeChat(chat.id);
    if (route.path === `/chat/${chat.id}`) go('/');
    toast('Chat deleted', { action: { label: 'Undo', run: () => restoreChat(removed) } });
  }
</script>

<Menu {placement} label="Chat actions" width="sm">
  {#snippet trigger(props)}
    {#if title}
      <Button variant="ghost" iconEnd={ChevronDown} {...props}>{chat.title}</Button>
    {:else}
      <IconButton icon={Ellipsis} label="More" {size} tooltip={false} {...props} />
    {/if}
  {/snippet}
  <MenuItem icon={chat.starred ? StarOff : Star} onselect={() => toggleStar(chat.id)}>{chat.starred ? 'Unstar' : 'Star'}</MenuItem>
  <MenuItem icon={Pencil}>Rename</MenuItem>
  <SubMenu icon={FolderInput} label="Add to project">
    {#each projects as p (p.id)}
      <MenuItem checked={chat.project === p.id} onselect={() => { chat.project = p.id; toast(`Moved to ${p.name}`); }}>{p.name}</MenuItem>
    {/each}
  </SubMenu>
  <MenuItem icon={Share} onselect={() => toast('Link copied')}>Share</MenuItem>
  <MenuSeparator />
  <MenuItem icon={Trash2} danger onselect={remove}>Delete</MenuItem>
</Menu>
