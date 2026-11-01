<script>
  import { Ellipsis, Pin, Pencil, Trash2 } from '@lucide/svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import IconButton from './IconButton.svelte';
  import { togglePin, removeChat, renameChat } from '../lib/app.svelte.js';
  import { prompt, confirmDialog } from '../lib/dialogs.svelte.js';

  let { chat, placement = 'bottom-start', size = 'xs', renamable = true, inline = false, onrename } = $props();

  async function rename() {
    if (onrename) return onrename();
    const next = await prompt({ title: 'Rename chat', label: 'Title', value: chat.title });
    if (next) renameChat(chat.id, next);
  }

  async function remove() {
    if (await confirmDialog({ title: 'Delete this chat?', body: `“${chat.title}” will be removed.`, confirm: 'Delete' })) removeChat(chat.id);
  }
</script>

{#if inline}
  {#if !chat.pinned}<IconButton icon={Pin} label="Pin" {size} onclick={() => togglePin(chat.id)} />{/if}
  <IconButton icon={Trash2} label="Delete chat" {size} onclick={remove} />
{:else}
<Menu {placement} label="Chat actions" width="sm">
  {#snippet trigger(props)}
    <IconButton icon={Ellipsis} label={`Actions for ${chat.title}`} {size} tooltip={false} {...props} />
  {/snippet}
  {#if !chat.pinned}<MenuItem icon={Pin} onselect={() => togglePin(chat.id)}>Pin</MenuItem>{/if}
  {#if renamable}<MenuItem icon={Pencil} onselect={rename}>Rename</MenuItem>{/if}
  <MenuSeparator />
  <MenuItem icon={Trash2} danger onselect={remove}>Delete</MenuItem>
</Menu>
{/if}
