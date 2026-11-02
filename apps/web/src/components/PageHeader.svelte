<script>
  import EditableTitle from './EditableTitle.svelte';

  let { title, size = 'md', leading, actions, onrename, renameLabel = 'Rename', children } = $props();
</script>

<header class={['header', size]}>
  {#if leading}<div class="leading">{@render leading()}</div>{/if}
  <div class="row">
    <h1>{#if onrename}<EditableTitle value={title} label={renameLabel} variant="heading" onsave={onrename} />{:else}{title}{/if}</h1>
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

  .leading {
    display: flex;
    align-items: center;
    min-height: var(--control-md);
  }

  .row {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-4);
    min-height: var(--control-lg);
  }

  h1 {
    flex: 1;
    min-width: 0;
    font-family: var(--font-serif);
    font-size: var(--text-2xl);
    font-weight: var(--weight-serif);
    line-height: var(--leading-tight);
    letter-spacing: var(--tracking-serif);
    overflow-wrap: anywhere;
  }

  .lg h1 {
    font-size: var(--text-3xl);
    letter-spacing: var(--tracking-tight);
  }

  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: none;
    min-height: var(--control-lg);
  }
</style>
