<script lang="ts">
  import { intercept } from '../router.svelte';
  let { href, label }: { href: string; label: string } = $props();
</script>

<a class="chip" {href} title={label} onclick={(e) => intercept(e, href)}>{label}</a>

<style>
  .chip {
    display: block;
    padding: var(--space-0-5) var(--space-1-5);
    border-radius: var(--radius-xs);
    background: color-mix(in srgb, var(--accent) 14%, var(--card));
    color: var(--foreground);
    font-size: var(--text-sm);
    text-decoration: none;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    transition: background-color var(--duration-fast) var(--ease-out);
  }
  .chip:hover {
    background: color-mix(in srgb, var(--accent) 24%, var(--card));
  }
</style>
