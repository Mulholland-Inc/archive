<script>
  import Icon from './Icon.svelte';

  // One row of a List: an icon and a line of text, a link when it has an href.
  let { href, icon, children } = $props();
</script>

<li>
  {#if href}
    <a class="row" {href}>{#if icon}<Icon {icon} size="sm" />{/if}{@render children()}</a>
  {:else}
    <span class="row">{#if icon}<Icon {icon} size="sm" />{/if}{@render children()}</span>
  {/if}
</li>

<style>
  .row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-height: var(--control-lg);
    padding: var(--space-1-5) var(--space-2);
    border-radius: var(--radius-md);
    color: var(--color-text-tertiary);
    font-size: var(--text-md);
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  a.row:hover {
    background: var(--color-fill-subtle);
  }
</style>
