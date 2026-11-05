<script>
  import Stack from './Stack.svelte';
  import Section from './Section.svelte';
  import Divider from './Divider.svelte';
  import ThemeCard from './ThemeCard.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import Card from './Card.svelte';
  import Prose from './Prose.svelte';
  import { theme, setTheme, setChatFont } from '../lib/theme.svelte.js';

  let chatFont = $state(theme.chatFont);

  const themes = [
    { value: 'light', label: 'Light' },
    { value: 'dark', label: 'Dark' },
    { value: 'system', label: 'System' }
  ];
  const fonts = [
    { value: 'sans', label: 'Sans' },
    { value: 'serif', label: 'Serif' }
  ];
  const sample = [{ type: 'p', text: 'The postmark reads **ENUGU 18 MR 74**, which matches the date at the top of the second letter.' }];
</script>

<Stack gap="8">
  <Section title="Theme" gap="lg">
    <div class="picker" role="radiogroup" aria-label="Theme">
      {#each themes as o (o.value)}
        <ThemeCard value={o.value} label={o.label} selected={theme.choice === o.value} onselect={setTheme} />
      {/each}
    </div>
  </Section>
  <Divider />
  <Section title="Chat font" gap="lg">
    <SegmentedControl bind:value={chatFont} options={fonts} label="Chat font" onchange={setChatFont} />
    <Card padding="lg" variant="filled"><Prose blocks={sample} /></Card>
  </Section>
</Stack>

<style>
  .picker {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--space-4);
    width: 100%;
    max-width: var(--dialog-lg);
  }
</style>
