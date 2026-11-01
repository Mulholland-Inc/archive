<script>
  import ThemeCard from './ThemeCard.svelte';

  let { value, onchange } = $props();
  const options = [
    { value: 'light', label: 'Light' },
    { value: 'dark', label: 'Dark' },
    { value: 'system', label: 'System' }
  ];
</script>

<div class="picker" role="radiogroup" aria-label="Theme">
  {#each options as o (o.value)}
    <ThemeCard value={o.value} label={o.label} selected={value === o.value} onselect={onchange} />
  {/each}
</div>

<style>
  .picker {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--space-4);
    width: 100%;
    max-width: var(--dialog-lg);
  }
</style>
