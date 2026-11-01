<script>
  import { Pin } from '@lucide/svelte';
  import IconButton from './IconButton.svelte';
  import { togglePin } from '../lib/app.svelte.js';

  let { chat, size = 'xs' } = $props();
</script>

<IconButton icon={Pin} label="Unpin" {size} onclick={() => togglePin(chat.id)} />
