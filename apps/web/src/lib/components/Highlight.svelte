<script lang="ts">
  let { text }: { text: string } = $props();
  const parts = $derived(text.split('**'));
</script>

<span class="highlight">{#each parts as p, i (i)}{#if i % 2}<mark>{p}</mark>{:else}{p}{/if}{/each}</span>

<style>
  .highlight {
    color: var(--muted-foreground);
    overflow-wrap: anywhere;
  }
  mark {
    padding: 0 var(--space-0-5);
    border-radius: var(--radius-xs);
    background: var(--highlight);
    color: var(--foreground);
  }
</style>
