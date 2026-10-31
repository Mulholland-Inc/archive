<script>
  import { getContext } from 'svelte';
  import { ChevronsUpDown, Plus, Settings } from '@lucide/svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuLabel from './MenuLabel.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import Avatar from './Avatar.svelte';
  import Icon from './Icon.svelte';

  let { workspaces = [], current, email, onselect, oncreate, onsettings } = $props();
  const sidebar = getContext('sidebar');
  const collapsed = $derived(sidebar?.collapsed ?? false);
  const active = $derived(workspaces.find((w) => w.id === current) ?? workspaces[0]);
</script>

<Menu placement="bottom-start" width="lg" label="Workspaces">
  {#snippet trigger(props)}
    <button type="button" class={['switcher', { collapsed }]} aria-label={`Workspace: ${active?.name}`} {...props}>
      <Avatar name={active?.name} tone={active?.tone} square size="sm" />
      <span class="name">{active?.name}</span>
      <span class="chev"><Icon icon={ChevronsUpDown} size="sm" /></span>
    </button>
  {/snippet}
  {#if email}<MenuLabel>{email}</MenuLabel>{/if}
  {#each workspaces as w (w.id)}
    <MenuItem checked={w.id === current} onselect={() => onselect?.(w.id)}>
      {#snippet leading()}<Avatar name={w.name} tone={w.tone} square size="sm" />{/snippet}
      <span class="ws">
        <span>{w.name}</span>
        <span class="kind">{w.kind} · {w.members} {w.members === 1 ? 'member' : 'members'}</span>
      </span>
    </MenuItem>
  {/each}
  <MenuSeparator />
  <MenuItem icon={Settings} onselect={onsettings}>Workspace settings</MenuItem>
  <MenuItem icon={Plus} onselect={oncreate}>New workspace</MenuItem>
</Menu>

<style>
  .switcher {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    min-width: 0;
    height: var(--control-lg);
    padding: var(--space-1-5);
    border-radius: var(--radius-md);
    text-align: start;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .switcher:hover,
  .switcher[aria-expanded='true'] {
    background: var(--color-fill);
  }

  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    transition: opacity var(--duration-normal) var(--ease-out);
  }

  .chev {
    display: inline-flex;
    color: var(--color-text-tertiary);
    transition: opacity var(--duration-normal) var(--ease-out);
  }

  .collapsed .name,
  .collapsed .chev {
    opacity: 0;
  }

  .ws {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .ws span {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .kind {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
  }
</style>
