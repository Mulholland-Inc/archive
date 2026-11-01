<script>
  import { ChevronLeft } from '@lucide/svelte';
  import IconButton from './IconButton.svelte';

  let { title, onback, backNarrow = false, tabs, tools, children } = $props();
</script>

<section class="pane">
  <header>
    <div class="lead">
      {#if onback}
        <span class={['back', { narrow: backNarrow }]}><IconButton icon={ChevronLeft} label="Back" tooltip={false} onclick={onback} /></span>
      {/if}
      <h2>{title}</h2>
      {#if tabs}{@render tabs()}{/if}
    </div>
    {#if tools}<div class="tools">{@render tools()}</div>{/if}
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
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-3) var(--space-4);
    padding: var(--space-5) var(--space-6) var(--space-4);
  }

  .lead {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    min-width: 0;
    min-height: var(--control-md);
  }

  h2 {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-sans);
    font-size: var(--text-xl);
    font-weight: var(--weight-medium);
    line-height: var(--leading-tight);
    letter-spacing: var(--tracking-snug);
  }

  .tools {
    display: flex;
    flex: 1 1 auto;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-2);
    min-width: 0;
  }

  .back {
    display: flex;
    margin-inline: calc(var(--space-2) * -1) calc(var(--space-2) * -1);
  }

  .narrow {
    display: none;
  }

  .body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: var(--space-2) var(--space-6) var(--space-6);
    container-type: inline-size;
  }

  @media (max-width: 56rem) {
    header {
      padding: var(--space-4) var(--space-4) var(--space-3);
    }

    .tools {
      flex: 1 1 100%;
    }

    .narrow {
      display: flex;
    }

    .body {
      padding: var(--space-2) var(--space-4) var(--space-4);
    }
  }
</style>
