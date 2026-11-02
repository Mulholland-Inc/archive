<script>
  import Stack from './Stack.svelte';
  import Section from './Section.svelte';
  import Divider from './Divider.svelte';
  import ThemePicker from './ThemePicker.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import Card from './Card.svelte';
  import Prose from './Prose.svelte';
  import { theme, setTheme, setChatFont } from '../lib/theme.svelte.js';

  let chatFont = $state(theme.chatFont);

  const fonts = [
    { value: 'sans', label: 'Sans' },
    { value: 'serif', label: 'Serif' }
  ];
  const sample = [{ type: 'p', text: 'The postmark reads **ENUGU 18 MR 74**, which matches the date at the top of the second letter.' }];
</script>

<Stack gap="8">
  <Section title="Theme" gap="lg">
    <ThemePicker value={theme.choice} onchange={setTheme} />
  </Section>
  <Divider />
  <Section title="Chat font" gap="lg">
    <SegmentedControl bind:value={chatFont} options={fonts} label="Chat font" onchange={setChatFont} />
    <Card padding="lg" variant="filled"><Prose blocks={sample} /></Card>
  </Section>
</Stack>
