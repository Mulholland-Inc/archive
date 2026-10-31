<script>
  import { CalendarClock, FilePlus, Mail, Hand } from '@lucide/svelte';
  import Icon from './Icon.svelte';
  import ConnectorIcon from './ConnectorIcon.svelte';

  let { trigger } = $props();
  const icons = { schedule: CalendarClock, file: FilePlus, email: Mail, manual: Hand };
</script>

<span class="trigger">
  {#if trigger.connector}
    <ConnectorIcon id={trigger.connector} size="xs" />
  {:else}
    <Icon icon={icons[trigger.kind] ?? Hand} size="sm" />
  {/if}
  <span>{trigger.label}</span>
</span>

<style>
  .trigger {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    min-width: 0;
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
  }

  span span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
