<script>
  import ProgressBar from './ProgressBar.svelte';
  import Text from './Text.svelte';

  // How many of a set are done, in words and as a bar beside them.
  let { done = 0, total = 1, label = 'Progress' } = $props();
</script>

<div class="count">
  <Text tone="secondary" tabular>{done} of {total} done</Text>
  <ProgressBar value={done / total} {label} />
</div>

<style>
  .count {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    max-width: var(--col-lg);
    white-space: nowrap;
  }

  .count > :global(.bar) {
    flex: 1;
  }
</style>
