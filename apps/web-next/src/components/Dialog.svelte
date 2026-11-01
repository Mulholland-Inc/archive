<script>
  import { X } from '@lucide/svelte';
  import IconButton from './IconButton.svelte';
  import { fade, pop } from '../lib/motion.js';
  import { uid } from '../lib/ids.js';

  let { open = $bindable(false), title, label, size = 'md', bare = false, footer, children, onclose } = $props();

  const titleId = uid('dlg');

  function close() {
    open = false;
    onclose?.();
  }

  function modal(node) {
    const previous = document.activeElement;
    const target = () => node.querySelector('[data-autofocus], .body input:not([type="checkbox"]):not([type="radio"]), .body textarea') ?? node.querySelector('.panel');
    node.showModal();
    target()?.focus();
    requestAnimationFrame(() => {
      if (!node.contains(document.activeElement) || document.activeElement.classList.contains('scrim')) target()?.focus();
    });
    const cancel = (event) => {
      event.preventDefault();
      close();
    };
    node.addEventListener('cancel', cancel);
    return {
      destroy() {
        node.removeEventListener('cancel', cancel);
        requestAnimationFrame(() => {
          if (previous?.isConnected && (!document.activeElement || document.activeElement === document.body)) previous.focus();
        });
      }
    };
  }
</script>

{#if open}
  <dialog use:modal class="dialog" aria-labelledby={title ? titleId : undefined} aria-label={title ? undefined : label}>
    <button type="button" class="scrim" tabindex="-1" aria-label="Close dialog" onclick={close} transition:fade={{ speed: 'normal' }}></button>
    <div class={['panel', size, { bare }]} tabindex="-1" transition:pop>
      {#if title}
        <header>
          <h2 id={titleId}>{title}</h2>
          <IconButton icon={X} label="Close" size="sm" tooltip={false} onclick={close} />
        </header>
      {/if}
      <div class="body">{@render children({ close })}</div>
      {#if footer}
        <footer>{@render footer({ close })}</footer>
      {/if}
    </div>
  </dialog>
{/if}

<style>
  .dialog {
    position: fixed;
    inset: 0;
    width: 100%;
    height: 100%;
    display: grid;
    place-items: center;
    padding: var(--space-6);
    overflow: hidden;
    background: transparent;
  }

  .scrim {
    position: absolute;
    inset: 0;
    background: var(--color-overlay);
    cursor: default;
  }

  .panel {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    width: 100%;
    max-width: var(--dialog-md);
    max-height: calc(100vh - var(--space-12));
    padding: var(--space-6);
    border-radius: var(--radius-xl);
    background: var(--color-surface-raised);
    box-shadow: var(--ring-strong);
    outline: none;
  }

  .sm { max-width: var(--dialog-sm); }
  .lg { max-width: var(--dialog-lg); }
  .xl { max-width: min(var(--dialog-xl), calc(100vw - var(--dialog-xl-margin))); height: min(var(--dialog-xl-height), calc(100vh - var(--dialog-xl-margin))); max-height: none; }
  .xl .body { flex: 1; gap: var(--space-0); overflow: hidden; }
  .palette { max-width: var(--palette-width); align-self: start; margin-block-start: 12vh; }

  .bare {
    padding: var(--space-0);
    gap: var(--space-0);
    overflow: hidden;
  }

  @media (max-width: 56rem) {
    .dialog:has(.xl) { padding: var(--space-0); }
    .xl { max-width: none; height: 100%; max-height: none; border-radius: var(--radius-none); box-shadow: none; }
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
  }

  h2 {
    font-family: var(--font-serif);
    font-size: var(--text-xl);
    font-weight: var(--weight-serif);
    line-height: var(--leading-tight);
    letter-spacing: var(--tracking-normal);
  }

  .body {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    min-height: 0;
    overflow-y: auto;
  }

  footer {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
  }
</style>
