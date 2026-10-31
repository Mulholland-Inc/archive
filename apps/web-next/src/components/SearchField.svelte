<script>
  import { Search, X } from '@lucide/svelte';
  import Input from './Input.svelte';
  import Kbd from './Kbd.svelte';
  import IconButton from './IconButton.svelte';

  let { value = $bindable(''), placeholder = 'Search', size = 'md', shortcut, label = 'Search', element = $bindable(), ...rest } = $props();
</script>

<Input bind:value bind:element type="search" icon={Search} {size} {placeholder} aria-label={label} {...rest}>
  {#snippet trailing()}
    {#if value}
      <IconButton icon={X} label="Clear" size="xs" tooltip={false} onclick={() => { value = ''; element?.focus(); }} />
    {:else if shortcut}
      <Kbd keys={shortcut} />
    {/if}
  {/snippet}
</Input>
