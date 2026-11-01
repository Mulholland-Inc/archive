<script>
  import { Plus } from '@lucide/svelte';
  import VaultRailItem from './VaultRailItem.svelte';
  import VaultIcon from './VaultIcon.svelte';
  import Icon from './Icon.svelte';

  let { vaults = [], current, onselect, oncreate } = $props();

  const running = (w) => w.workflows.filter((f) => f.runs?.[0] && ['running', 'waiting'].includes(f.runs[0].status)).length;
</script>

<nav class="rail" aria-label="Vaults">
  {#each vaults as w (w.id)}
    <VaultRailItem label={w.name} active={w.id === current} count={running(w)} onclick={() => onselect?.(w.id)}>
      <VaultIcon icon={w.look.icon} tint={w.look.tint} seed={w.id} size="rail" />
    </VaultRailItem>
  {/each}
  <VaultRailItem label="New vault" add onclick={oncreate}>
    <Icon icon={Plus} />
  </VaultRailItem>
</nav>

<style>
  .rail {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    flex: none;
    width: var(--vault-rail);
    height: 100%;
    padding-block: var(--space-3);
    background: var(--color-rail);
    box-shadow: inset calc(var(--border-width) * -1) 0 0 var(--color-border-subtle);
    transition: background-color var(--duration-normal) var(--ease-out);
    overflow-y: auto;
    overflow-x: hidden;
    scrollbar-width: none;
  }
</style>
