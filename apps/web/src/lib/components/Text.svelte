<script lang="ts">
  import type { Snippet } from 'svelte';
  let {
    variant = 'body',
    as = 'span',
    truncate = false,
    tone,
    children
  }: {
    variant?: 'body' | 'strong' | 'muted' | 'subtle' | 'mono' | 'code' | 'small' | 'heading' | 'title';
    as?: 'span' | 'p' | 'h1' | 'h2' | 'h3' | 'div';
    truncate?: boolean;
    tone?: 'success' | 'warning' | 'danger' | 'accent' | 'muted';
    children: Snippet;
  } = $props();
</script>

<svelte:element this={as} class="text" data-variant={variant} data-truncate={truncate || undefined} data-tone={tone}>{@render children()}</svelte:element>

<style>
  .text {
    font-size: inherit;
    font-weight: var(--font-normal);
    line-height: var(--leading-normal);
    color: inherit;
    min-width: 0;
  }
  [data-truncate] {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  [data-tone='success'] { color: var(--success); }
  [data-tone='warning'] { color: var(--warning); }
  [data-tone='danger'] { color: var(--danger); }
  [data-tone='accent'] { color: var(--accent); }
  [data-tone='muted'] { color: var(--muted-foreground); }
  [data-variant='strong'] { font-weight: var(--font-medium); }
  [data-variant='muted'] { color: var(--muted-foreground); }
  [data-variant='subtle'] { color: var(--subtle-foreground); }
  [data-variant='small'] {
    font-size: var(--text-sm);
    color: var(--muted-foreground);
  }
  [data-variant='mono'] {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    letter-spacing: var(--tracking-mono);
    color: var(--subtle-foreground);
  }
  [data-variant='code'] {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    letter-spacing: var(--tracking-mono);
  }
  [data-variant='heading'] {
    font-size: var(--text-lg);
    font-weight: var(--font-semibold);
    letter-spacing: var(--tracking-snug);
    line-height: var(--leading-tight);
  }
  [data-variant='title'] {
    font-family: var(--font-display);
    font-size: var(--text-2xl);
    font-weight: var(--font-semibold);
    letter-spacing: var(--tracking-tight);
    line-height: var(--leading-tight);
  }
</style>
