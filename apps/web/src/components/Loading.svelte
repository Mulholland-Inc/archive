<script>
  import Spinner from './Spinner.svelte';

  // The whole window while the app starts: a spinner in its middle.
</script>

<div class="loading" aria-label="Loading"><Spinner /></div>

<style>
  .loading {
    display: grid;
    place-items: center;
    min-height: 100dvh;
    color: var(--color-text-tertiary);
  }
</style>
