<script lang="ts">
  import { intercept } from '../router.svelte';
  let { href, label, onclick }: { href: string; label: string; onclick?: () => void } = $props();
</script>

<a class="chip" {href} title={label} onclick={(e) => {
  if (onclick && e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey) {
    e.preventDefault();
    onclick();
  } else intercept(e, href);
}}>{label}</a>

<style>
  .chip {
    display: block;
    padding: var(--space-0-5) var(--space-1-5);
    border-radius: var(--radius-xs);
    background: var(--tint-2);
    color: var(--foreground);
    font-size: var(--text-sm);
    text-decoration: none;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    transition: background-color var(--duration-fast) var(--ease-out);
  }
  .chip:hover {
    background: var(--tint-3);
  }
</style>
