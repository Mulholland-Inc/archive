<script>
  let { text } = $props();

  const parts = $derived(text.split('`'));
</script>

{#each parts as part, i}{#if i % 2}<code class="inline">{part}</code>{:else}{part}{/if}{/each}
