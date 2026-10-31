<script>
  import { getContext } from 'svelte';
  import { ChevronsUpDown, Plus, Settings } from '@lucide/svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import WorkspaceIcon from './WorkspaceIcon.svelte';
  import Icon from './Icon.svelte';

  let { workspaces = [], current, onselect, oncreate, onsettings } = $props();
  const sidebar = getContext('sidebar');
  const collapsed = $derived(sidebar?.collapsed ?? false);
  const active = $derived(workspaces.find((w) => w.id === current) ?? workspaces[0]);
</script>

<Menu placement="bottom-start" width="lg" label="Workspaces">
  {#snippet trigger(props)}
    <button type="button" class={['switcher', { collapsed }]} aria-label={`Workspace: ${active?.name}`} {...props}>
      <WorkspaceIcon icon={active?.look.icon} tint={active?.look.tint} seed={active?.id} size="lg" />
      <span class="lines">
        <span class="name">{active?.name}</span>
        <span class="sub">{active?.kind} · {active?.members.length} {active?.members.length === 1 ? 'member' : 'members'}</span>
      </span>
      <span class="chev"><Icon icon={ChevronsUpDown} size="sm" /></span>
    </button>
  {/snippet}
  {#each workspaces as w (w.id)}
    <MenuItem checked={w.id === current} onselect={() => onselect?.(w.id)}>
      {#snippet leading()}<WorkspaceIcon icon={w.look.icon} tint={w.look.tint} seed={w.id} size="md" />{/snippet}
      <span class="ws">
        <span>{w.name}</span>
        <span class="kind">{w.kind} · {w.members.length} {w.members.length === 1 ? 'member' : 'members'}</span>
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
    gap: var(--space-3);
    padding: var(--space-2);
    border-radius: var(--radius-lg);
    text-align: start;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      transform var(--duration-instant) var(--ease-out);
  }

  .switcher:active {
    transform: scale(var(--scale-press));
  }

  .collapsed {
    padding: var(--space-0);
    gap: var(--space-0);
  }

  .lines {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    line-height: var(--leading-snug);
    transition: opacity var(--duration-normal) var(--ease-out);
  }

  .sub {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
  }

  .switcher:hover,
  .switcher[aria-expanded='true'] {
    background: var(--color-fill);
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
  }

  .chev {
    display: inline-flex;
    color: var(--color-text-tertiary);
    transition: opacity var(--duration-normal) var(--ease-out);
  }

  .collapsed .lines,
  .collapsed .chev {
    display: none;
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
