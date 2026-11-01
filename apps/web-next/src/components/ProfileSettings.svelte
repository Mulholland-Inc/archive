<script>
  import Stack from './Stack.svelte';
  import Grid from './Grid.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Textarea from './Textarea.svelte';
  import Select from './Select.svelte';
  import Avatar from './Avatar.svelte';
  import Button from './Button.svelte';
  import { me } from '../lib/data.js';
  import { app } from '../lib/app.svelte.js';
  import { toast } from '../lib/toast.svelte.js';

  let name = $state(app.profile.name);
  let call = $state(app.profile.first);
  let work = $state(app.profile.work);
  let prefs = $state(app.profile.prefs);
  let tone = $state(app.profile.tone);

  const dirty = $derived(name !== app.profile.name || call !== app.profile.first || work !== app.profile.work || prefs !== app.profile.prefs || tone !== app.profile.tone);

  const works = [
    { value: 'archivist', label: 'Family historian' },
    { value: 'legal', label: 'Legal' },
    { value: 'design', label: 'Design' },
    { value: 'research', label: 'Research' },
    { value: 'other', label: 'Something else' }
  ];

  function save() {
    Object.assign(app.profile, { name: name.trim() || app.profile.name, first: call.trim() || app.profile.first, work, prefs, tone });
    toast('Profile saved', { tone: 'success' });
  }

  function reset() {
    name = app.profile.name;
    call = app.profile.first;
    work = app.profile.work;
    prefs = app.profile.prefs;
    tone = app.profile.tone;
  }
</script>

<Stack gap="6">
  <Stack direction="row" gap="4" align="center">
    <Avatar {name} {tone} size="xl" />
    <Stack direction="row" gap="2">
      <Button size="sm" onclick={() => (tone = (tone % 6) + 1)}>Change colour</Button>
      <Button size="sm" variant="ghost" disabled={tone === me.tone} onclick={() => (tone = me.tone)}>Reset</Button>
    </Stack>
  </Stack>
  <Grid columns="2" gap="4">
    <Field label="Full name">{#snippet children(id)}<Input {id} bind:value={name} />{/snippet}</Field>
    <Field label="What should Archive call you?">{#snippet children(id)}<Input {id} bind:value={call} />{/snippet}</Field>
  </Grid>
  <Field label="What best describes your work?">{#snippet children(id)}<Select {id} bind:value={work} options={works} full />{/snippet}</Field>
  <Field label="Personal preferences">{#snippet children(id)}<Textarea {id} bind:value={prefs} rows={4} />{/snippet}</Field>
  <Stack direction="row" gap="2" justify="end">
    <Button variant="ghost" disabled={!dirty} onclick={reset}>Cancel</Button>
    <Button variant="primary" disabled={!dirty} onclick={save}>Save changes</Button>
  </Stack>
</Stack>
