<script>
  import { Ellipsis, Pin, Trash2 } from '@lucide/svelte';
  import Menu from './Menu.svelte';
  import MenuItems from './MenuItems.svelte';
  import IconButton from './IconButton.svelte';
  import { togglePin } from '../lib/app.svelte.js';
  import { chatItems, deleteChat, renameChatPrompt } from '../lib/menus.js';

  let { chat, placement = 'bottom-start', size = 'xs', renamable = true, inline = false, onrename } = $props();
</script>

{#if inline}
  {#if !chat.pinned}<IconButton icon={Pin} label="Pin" {size} onclick={() => togglePin(chat.id)} />{/if}
  <IconButton icon={Trash2} label="Delete chat" {size} onclick={() => deleteChat(chat)} />
{:else}
<Menu {placement} label="Chat actions" width="sm">
  {#snippet trigger(props)}
    <IconButton icon={Ellipsis} label={`Actions for ${chat.title}`} {size} tooltip={false} {...props} />
  {/snippet}
  <MenuItems items={chatItems(chat, { open: false, unpin: false, rename: renamable && (onrename ?? (() => renameChatPrompt(chat))) })} />
</Menu>
{/if}
