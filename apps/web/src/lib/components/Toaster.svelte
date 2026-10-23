<script lang="ts">
  import { flip } from 'svelte/animate';
  import Icon from './Icon.svelte';
  import { toasts, dismiss } from '../toast.svelte';
  import { ms, ease } from '../motion';

  function enter(_node: Element) {
    return {
      duration: ms('slow'),
      easing: ease('spring'),
      css: (t: number) => `opacity:${Math.min(1, t * 1.5)};transform:translateY(${(1 - t) * 1}rem) scale(${0.96 + 0.04 * t})`
    };
  }
  function leave(_node: Element) {
    return { duration: ms('normal'), easing: ease('in'), css: (t: number) => `opacity:${t};transform:scale(${0.96 + 0.04 * t})` };
  }
</script>

<div class="toaster" aria-live="polite">
  {#each toasts as t (t.id)}
    <div class="toast" data-tone={t.tone} in:enter out:leave animate:flip={{ duration: ms('normal'), easing: ease('out') }}>
      <span class="icon"><Icon name={t.tone === 'danger' ? 'alert' : t.tone === 'success' ? 'check' : 'bell'} /></span>
      <span class="message">{t.message}</span>
      {#if t.action}
        <button
          class="action"
          onclick={() => {
            t.action?.run();
            dismiss(t.id);
          }}>{t.action.label}</button
        >
      {/if}
      <button class="close" aria-label="Dismiss" onclick={() => dismiss(t.id)}><Icon name="x" size="sm" /></button>
    </div>
  {/each}
</div>

<style>
  .toaster {
    position: fixed;
    right: var(--space-4);
    bottom: var(--space-4);
    z-index: var(--z-toast);
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: var(--space-2);
    pointer-events: none;
  }
  .toast {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2-5);
    width: var(--toast-width);
    max-width: calc(100vw - var(--space-8));
    padding: var(--space-3);
    border-radius: var(--radius-lg);
    background: var(--popover);
    color: var(--popover-foreground);
    box-shadow: var(--shadow-popover);
    pointer-events: auto;
  }
  .icon {
    display: grid;
    padding-top: var(--space-0-5);
    color: var(--muted-foreground);
  }
  [data-tone='success'] .icon {
    color: var(--success);
  }
  [data-tone='danger'] .icon {
    color: var(--danger);
  }
  .message {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .action {
    font-weight: var(--font-medium);
    color: var(--accent);
  }
  .close {
    display: grid;
    padding: var(--space-0-5);
    border-radius: var(--radius-xs);
    color: var(--muted-foreground);
  }
  .close:hover {
    color: var(--foreground);
  }
</style>
