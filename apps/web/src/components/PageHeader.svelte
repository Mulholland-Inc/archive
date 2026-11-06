<script>
  import EditableTitle from './EditableTitle.svelte';
  import Heading from './Heading.svelte';

  let { title, size = 'md', actions, onrename, renameLabel = 'Rename', children } = $props();
</script>

<header class={['header', size]}>
  <div class="row">
    <Heading level={1} size={size === 'lg' ? '2xl' : 'xl'} tracking={size === 'lg' ? 'tight' : undefined} wrap="anywhere" grow>{#if onrename}<EditableTitle value={title} label={renameLabel} variant="heading" onsave={onrename} />{:else}{title}{/if}</Heading>
    {#if actions}<div class="actions">{@render actions()}</div>{/if}
  </div>
  {#if children}{@render children()}{/if}
</header>

<style>
  .header {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    min-width: 0;
  }

  .row {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-4);
    min-height: var(--control-lg);
  }

  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: none;
    min-height: var(--control-lg);
  }
</style>
