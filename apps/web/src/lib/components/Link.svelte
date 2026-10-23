<script lang="ts">
  import type { Snippet } from 'svelte';
  import { intercept } from '../router.svelte';
  let {
    href,
    variant = 'plain',
    external = false,
    children
  }: { href: string; variant?: 'plain' | 'subtle' | 'accent' | 'block'; external?: boolean; children: Snippet } = $props();
</script>

<a
  class="link"
  data-variant={variant}
  {href}
  target={external ? '_blank' : undefined}
  rel={external ? 'noreferrer' : undefined}
  onclick={(e) => !external && intercept(e, href)}>{@render children()}</a
>

<style>
  .link {
    color: inherit;
    text-decoration: none;
    min-width: 0;
    border-radius: var(--radius-xs);
    transition: color var(--duration-fast) var(--ease-out);
  }
  [data-variant='plain']:hover {
    text-decoration: underline;
    text-decoration-color: var(--tint-4);
    text-underline-offset: 0.2em;
  }
  [data-variant='subtle'] {
    color: var(--muted-foreground);
  }
  [data-variant='subtle']:hover {
    color: var(--foreground);
  }
  [data-variant='accent'] {
    color: var(--accent);
  }
  [data-variant='accent']:hover {
    text-decoration: underline;
    text-underline-offset: 0.2em;
  }
  [data-variant='block'] {
    display: block;
  }
</style>
