<script>
  import { ChevronDown } from '@lucide/svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import Icon from './Icon.svelte';
  import { models } from '../lib/mock/data.js';

  let { model = $bindable('pro'), thinking = $bindable(true), size = 'md' } = $props();
  const current = $derived(models.find((m) => m.id === model));
</script>

<Menu placement="top-end" label="Model">
  {#snippet trigger(props)}
    <button type="button" class={['trigger', size]} {...props}>
      <span class="name">{current?.name}</span>
      {#if thinking}<span class="mode">Extended</span>{/if}
      <Icon icon={ChevronDown} size="xs" />
    </button>
  {/snippet}
  {#each models as m (m.id)}
    <MenuItem checked={m.id === model} onselect={() => (model = m.id)}>{m.name}</MenuItem>
  {/each}
  <MenuSeparator />
  <MenuItem keepOpen toggle={thinking} onselect={() => (thinking = !thinking)}>Extended thinking</MenuItem>
</Menu>

<style>
  .trigger {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    height: var(--control-md);
    padding: var(--space-2);
    border-radius: var(--radius-md);
    color: var(--color-text);
    font-size: var(--text-md);
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .sm {
    height: var(--control-sm);
    padding: var(--space-1-5);
    font-size: var(--text-sm);
  }

  .trigger:hover,
  .trigger[aria-expanded='true'] {
    background: var(--color-fill);
  }

  .name {
    font-weight: var(--weight-medium);
  }

  .mode {
    color: var(--color-text-tertiary);
  }
</style>
