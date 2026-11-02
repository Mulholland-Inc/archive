<script>
  import { pinned } from '../lib/pagetitle.svelte.js';

  let { leading, trailing, children, bordered = false, mobile = false } = $props();

  let text = $state('');
  $effect(() => {
    if (pinned.title) text = pinned.title;
  });
</script>

<header class={['topbar', { bordered, mobile }]}>
  <div class="side">{@render leading?.()}</div>
  <div class="center">
    {#if children}{@render children()}{:else if mobile}<span class={['title', { shown: pinned.title }]} aria-hidden={!pinned.title}>{text}</span>{/if}
  </div>
  <div class="side end">{@render trailing?.()}</div>
</header>

<style>
  .topbar {
    position: sticky;
    inset-block-start: 0;
    z-index: var(--z-sticky);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: none;
    height: var(--topbar-height);
    padding: var(--space-2);
    background: var(--color-canvas);
    transition: background-color var(--duration-normal) var(--ease-out);
  }

  @media (min-width: 56rem) {
    .mobile {
      display: none;
    }
  }

  .bordered {
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  .title {
    padding-inline: var(--space-1);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-serif);
    font-size: var(--text-xl);
    font-weight: var(--weight-serif);
    letter-spacing: var(--tracking-serif);
    line-height: var(--leading-tight);
    opacity: 0;
    transform: translateY(var(--shift-sm));
    transition:
      opacity var(--duration-normal) var(--ease-out),
      transform var(--duration-normal) var(--ease-out);
  }

  .title.shown {
    opacity: 1;
    transform: none;
  }

  .side {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    min-width: 0;
  }

  .center {
    display: flex;
    align-items: center;
    flex: 1;
    min-width: 0;
  }

  .end {
    flex: none;
    justify-content: flex-end;
  }
</style>
