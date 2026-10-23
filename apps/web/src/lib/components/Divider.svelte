<script lang="ts">
  let { vertical = false }: { vertical?: boolean } = $props();
</script>

<div class="divider" data-vertical={vertical || undefined} role="separator"></div>

<style>
  .divider {
    flex: none;
    height: var(--border-width);
    background: var(--border);
  }
  [data-vertical] {
    width: var(--border-width);
    height: var(--icon-size);
    align-self: center;
  }
</style>
