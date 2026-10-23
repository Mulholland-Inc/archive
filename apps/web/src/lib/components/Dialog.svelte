<script lang="ts">
  import type { Snippet } from 'svelte';
  import IconButton from './IconButton.svelte';
  import { ms, ease } from '../motion';
  let {
    open = $bindable(false),
    title,
    description,
    size = 'md',
    footer,
    children,
    onclose
  }: {
    open?: boolean;
    title: string;
    description?: string;
    size?: 'md' | 'lg';
    footer?: Snippet;
    children: Snippet;
    onclose?: () => void;
  } = $props();

  let panel: HTMLElement | undefined = $state();
  let previous: Element | null = null;

  function close() {
    open = false;
    onclose?.();
  }

  $effect(() => {
    if (!open || !panel) return;
    previous = document.activeElement;
    const focusable = () =>
      [...panel!.querySelectorAll<HTMLElement>('input,textarea,button,[tabindex]:not([tabindex="-1"]),a[href]')].filter(
        (el) => !el.hasAttribute('disabled')
      );
    const first = focusable().find((el) => el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') ?? focusable()[1] ?? focusable()[0];
    first?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) {
        e.preventDefault();
        close();
      } else if (e.key === 'Tab') {
        const list = focusable();
        if (!list.length) return;
        const i = list.indexOf(document.activeElement as HTMLElement);
        if (e.shiftKey && i <= 0) {
          e.preventDefault();
          list[list.length - 1].focus();
        } else if (!e.shiftKey && i === list.length - 1) {
          e.preventDefault();
          list[0].focus();
        }
      }
    };
    document.addEventListener('keydown', onKey);
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.style.overflow = '';
      (previous as HTMLElement | null)?.focus?.();
    };
  });

  function backdrop(_node: Element) {
    return { duration: ms('normal'), easing: ease('out'), css: (t: number) => `opacity:${t}` };
  }

  function rise(_node: Element) {
    return {
      duration: ms('slow'),
      easing: ease('out'),
      css: (t: number) => `opacity:${t};transform:translateY(${(1 - t) * 0.75}rem) scale(${0.97 + 0.03 * t})`
    };
  }
</script>

{#if open}
  <div class="overlay" transition:backdrop role="presentation" onpointerdown={(e) => e.target === e.currentTarget && close()}>
    <div class="dialog" data-size={size} role="dialog" aria-modal="true" aria-labelledby="dialog-title" bind:this={panel} transition:rise>
      <header class="header">
        <div class="titles">
          <h2 class="title" id="dialog-title">{title}</h2>
          {#if description}<p class="description">{description}</p>{/if}
        </div>
        <IconButton icon="x" label="Close" onclick={close} />
      </header>
      <div class="body">{@render children()}</div>
      {#if footer}<footer class="footer">{@render footer()}</footer>{/if}
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: var(--z-dialog);
    display: grid;
    place-items: start center;
    padding: var(--dialog-offset) var(--space-4) var(--space-4);
    background: var(--overlay);
    overflow-y: auto;
  }
  .dialog {
    display: flex;
    flex-direction: column;
    width: 100%;
    max-width: var(--dialog-width);
    border-radius: var(--dialog-radius);
    background: var(--popover);
    color: var(--popover-foreground);
    box-shadow: var(--shadow-lift);
  }
  [data-size='lg'] {
    max-width: var(--dialog-width-lg);
  }
  .header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-5) var(--space-5) 0;
  }
  .titles {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding-top: var(--space-0-5);
  }
  .title {
    font-size: var(--text-lg);
    font-weight: var(--font-semibold);
    letter-spacing: var(--tracking-snug);
    line-height: var(--leading-tight);
  }
  .description {
    color: var(--muted-foreground);
  }
  .body {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding: var(--space-5);
  }
  .footer {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    padding: 0 var(--space-5) var(--space-5);
  }
  @media (max-width: 40rem) {
    .overlay {
      padding: var(--space-3);
      place-items: end center;
    }
    .header {
      padding: var(--space-4) var(--space-4) 0;
    }
    .body {
      padding: var(--space-4);
    }
    .footer {
      flex-wrap: wrap;
      padding: 0 var(--space-4) var(--space-4);
    }
  }
</style>
