<script>
  import { ChevronLeft, X } from '@lucide/svelte';
  import IconButton from './IconButton.svelte';

  let { title, titleId, onback, backNarrow = false, onclose, children } = $props();
</script>

<section class="pane">
  <header>
    {#if onback}
      <span class={['back', { narrow: backNarrow }]}><IconButton icon={ChevronLeft} label="Back" size="sm" tooltip={false} onclick={onback} /></span>
    {/if}
    <h2 id={titleId}>{title}</h2>
    {#if onclose}<IconButton icon={X} label="Close" size="sm" tooltip={false} onclick={onclose} />{/if}
  </header>
  <div class="body">{@render children()}</div>
</section>

<style>
  .pane {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    height: 100%;
    background: var(--color-surface-raised);
  }

  header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-4) var(--space-5);
    border-block-end: var(--border-width) solid var(--color-border-subtle);
  }

  h2 {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-serif);
    font-size: var(--text-lg);
    font-weight: var(--weight-serif);
    line-height: var(--leading-tight);
  }

  .back {
    display: flex;
    margin-inline-start: calc(var(--space-2) * -1);
  }

  .narrow {
    display: none;
  }

  .body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: var(--space-5);
    container-type: inline-size;
  }

  @media (max-width: 56rem) {
    header {
      padding: var(--space-3) var(--space-4);
    }

    .narrow {
      display: flex;
    }

    .body {
      padding: var(--space-4);
    }
  }
</style>
