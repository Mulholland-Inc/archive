<script>
  import Section from './Section.svelte';
  import ThemeCard from './ThemeCard.svelte';
  import { theme, setTheme } from '../lib/theme.svelte.js';

  const themes = [
    { value: 'light', label: 'Light' },
    { value: 'dark', label: 'Dark' },
    { value: 'system', label: 'System' }
  ];
</script>

<Section title="Theme" gap="lg">
  <div class="picker" role="radiogroup" aria-label="Theme">
    {#each themes as o (o.value)}
      <ThemeCard value={o.value} label={o.label} selected={theme.choice === o.value} onselect={setTheme} />
    {/each}
  </div>
</Section>

<style>
  .picker {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--space-4);
    width: 100%;
    max-width: var(--dialog-lg);
  }
</style>
