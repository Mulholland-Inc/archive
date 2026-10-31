<script>
  import { CircleCheck, CircleAlert, Info, X } from '@lucide/svelte';
  import { flip } from 'svelte/animate';
  import Icon from './Icon.svelte';
  import { toasts, dismiss } from '../lib/toast.svelte.js';
  import { toastIn, fade, duration, ease } from '../lib/motion.js';

  const icons = { success: CircleCheck, danger: CircleAlert, neutral: Info };
</script>

<div class="region" role="status" aria-live="polite">
  {#each toasts as t (t.id)}
    <div class={['toast', t.tone]} in:toastIn out:fade animate:flip={{ duration: duration('normal'), easing: ease('out') }}>
      <Icon icon={icons[t.tone] ?? Info} size="md" />
      <span class="message">{t.message}</span>
      {#if t.action}
        <button type="button" class="action" onclick={() => { t.action.run(); dismiss(t.id); }}>{t.action.label}</button>
      {/if}
      <button type="button" class="close" aria-label="Dismiss" onclick={() => dismiss(t.id)}><Icon icon={X} size="sm" /></button>
    </div>
  {/each}
</div>

<style>
  .region {
    position: fixed;
    inset-block-end: var(--space-6);
    inset-inline-end: var(--space-6);
    z-index: var(--z-toast);
    display: flex;
    flex-direction: column-reverse;
    align-items: flex-end;
    gap: var(--space-2);
    pointer-events: none;
  }

  .toast {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    max-width: var(--toast-width);
    padding: var(--space-2);
    border-radius: var(--radius-lg);
    background: var(--color-inverse);
    color: var(--color-text-inverse);
    box-shadow: var(--shadow-lg);
    font-size: var(--text-md);
    pointer-events: auto;
  }

  .toast > :global(.icon) {
    margin-inline-start: var(--space-1);
    opacity: 0.7;
  }

  .success > :global(.icon) {
    opacity: 1;
    color: var(--color-success-inverse);
  }

  .danger > :global(.icon) {
    opacity: 1;
    color: var(--color-danger-inverse);
  }

  .message {
    flex: 1;
    padding: var(--space-1);
  }

  .action {
    height: var(--control-sm);
    padding: var(--space-1-5);
    border-radius: var(--radius-sm);
    font-weight: var(--weight-semibold);
  }

  .action:hover,
  .close:hover {
    background: var(--color-inverse-fill);
  }

  .close {
    display: grid;
    place-items: center;
    width: var(--control-sm);
    height: var(--control-sm);
    border-radius: var(--radius-sm);
    opacity: 0.7;
  }
</style>
