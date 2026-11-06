<script>
  import Spinner from './Spinner.svelte';
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
