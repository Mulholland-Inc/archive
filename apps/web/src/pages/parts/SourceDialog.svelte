<script lang="ts">
  import { Dialog, Field, Input, Textarea, Select, Button, Stack, Notice, ChoiceCard, Grid, Checkbox, Row, Text } from '../../lib';
  import { api, ApiError } from '../../lib/api';
  import { session, spaceLabel } from '../../lib/session.svelte';
  import { objectTypes, humanize } from '../../lib/spec';
  import { slug } from '../../lib/ontology';
  import { kinds, schedules } from '../../lib/sources';
  import type { Source } from '../../lib/types';

  let { open = $bindable(false), onadded }: { open?: boolean; onadded: (s: Source) => void } = $props();

  let kind = $state('');
  let name = $state('');
  let nameTouched = $state(false);
  let space = $state('');
  let type = $state('');
  let schedule = $state('1h');
  let path = $state('');
  let host = $state('');
  let username = $state('');
  let password = $state('');
  let mailboxes = $state('INBOX');
  let starttls = $state(false);
  let command = $state('');
  let protocol = $state('');
  let extra = $state('{}');
  let errors = $state<{ [k: string]: string }>({});
  let failure = $state('');
  let busy = $state(false);

  const writable = $derived(session.spaces.filter((s) => ['editor', 'admin', 'owner'].includes(s.level)));

  $effect(() => {
    if (open) {
      kind = '';
      name = '';
      nameTouched = false;
      type = '';
      schedule = '1h';
      path = host = username = password = command = protocol = '';
      mailboxes = 'INBOX';
      starttls = false;
      extra = '{}';
      errors = {};
      failure = '';
      space = (writable.find((s) => s.name === session.space) ?? writable.find((s) => s.id === session.home) ?? writable[0])?.name ?? '';
    }
  });

  $effect(() => {
    if (!nameTouched && kind) name = slug(kind === 'folder' && path ? path.split('/').filter(Boolean).pop() ?? kind : kind).replace(/_/g, '-');
  });

  async function submit(e?: SubmitEvent) {
    e?.preventDefault();
    errors = {};
    failure = '';
    const config: { [k: string]: unknown } = {};
    const credentials: { [k: string]: unknown } = {};
    if (kind === 'folder') config.path = path.trim();
    if (kind === 'imap') {
      Object.assign(config, { host: host.trim(), username: username.trim(), mailboxes: mailboxes.split(',').map((m) => m.trim()).filter(Boolean), starttls });
      credentials.password = password;
    }
    if (kind === 'external') {
      config.command = command.trim().split(/\s+/).filter(Boolean);
      if (protocol) config.protocol = protocol;
      try {
        const parsed = JSON.parse(extra || '{}');
        if (Object.keys(parsed).length) config.config = parsed;
      } catch {
        errors['config.config'] = 'This is not valid JSON.';
        return;
      }
    }
    busy = true;
    try {
      const s = await api.addSource({ kind, name, space: space || undefined, type: type || undefined, schedule, config, credentials });
      if (s.auth_url) {
        location.href = s.auth_url;
        return;
      }
      open = false;
      onadded(s);
    } catch (err) {
      if (err instanceof ApiError && err.fields.length) {
        for (const f of err.fields) errors[f.path] = f.message;
        if (!err.fields.some((f) => ['name', 'config.path', 'config.host', 'config.username', 'credentials.password', 'schedule', 'type'].includes(f.path))) failure = err.message;
      } else failure = (err as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<Dialog bind:open title="Connect a source" description="Archive copies new items into raw storage, then reads them into the type you choose." size="lg">
  <form onsubmit={submit} novalidate>
    <Stack gap="4">
      {#if failure}<Notice tone="danger">{failure}</Notice>{/if}
      <Grid min="sm">
        {#each kinds as k (k.id)}
          <ChoiceCard icon={k.icon} title={k.label} description={k.description} selected={kind === k.id} onclick={() => (kind = k.id)} />
        {/each}
      </Grid>
      {#if kind}
        {#if kind === 'folder'}
          <Field label="Folder path" hint="An absolute path on the Archive server." error={errors['config.path']}>
            <Input bind:value={path} placeholder="/home/me/Documents/Receipts" mono invalid={!!errors['config.path']} autofocus label="Folder path" />
          </Field>
        {/if}
        {#if kind === 'imap'}
          <Field label="Server" hint="host:port" error={errors['config.host']}>
            <Input bind:value={host} placeholder="imap.fastmail.com:993" mono invalid={!!errors['config.host']} autofocus label="Server" />
          </Field>
          <Field label="Username" error={errors['config.username']}>
            <Input bind:value={username} autocomplete="off" invalid={!!errors['config.username']} label="Username" />
          </Field>
          <Field label="App password" hint="Stored encrypted. Use an app password for iCloud, Outlook and Fastmail." error={errors['credentials.password']}>
            <Input bind:value={password} type="password" autocomplete="new-password" invalid={!!errors['credentials.password']} label="App password" />
          </Field>
          <Field label="Mailboxes" hint="Separate several with commas.">
            <Input bind:value={mailboxes} label="Mailboxes" />
          </Field>
          <Row gap="2"><Checkbox checked={starttls} label="Use STARTTLS" onchange={(v) => (starttls = v)} /><Text>Use STARTTLS instead of TLS</Text></Row>
        {/if}
        {#if kind === 'external'}
          <Field label="Command" hint="The program and its arguments, run on the Archive server." error={errors['config.command']}>
            <Input bind:value={command} placeholder="tap-github" mono autofocus label="Command" />
          </Field>
          <Field label="Protocol">
            <Select variant="field" bind:value={protocol} options={[{ id: '', label: 'Archive JSON lines' }, { id: 'singer', label: 'Singer tap' }]} label="Protocol" />
          </Field>
          <Field label="Connector settings" hint="JSON passed to the connector." error={errors['config.config']}>
            <Textarea bind:value={extra} rows={4} mono label="Connector settings" />
          </Field>
        {/if}
        {#if kind === 'gmail' || kind === 'drive'}
          <Notice>Next, Google asks you to allow read-only access. Archive keeps the sign-in encrypted and comes back here when you are done.</Notice>
        {/if}
        <Field label="Name" hint="Lowercase, used in search filters and the CLI." error={errors.name}>
          <Input bind:value={name} mono invalid={!!errors.name} oninput={() => (nameTouched = true)} label="Name" />
        </Field>
        <Field label="Read into" hint="Without a type, items stay searchable as unmapped raw data." error={errors.type}>
          <Select variant="field" bind:value={type} options={[{ id: '', label: 'Keep them unmapped' }, ...objectTypes(session.ontology).map((t) => ({ id: t, label: humanize(t), icon: 'table' }))]} label="Read into" />
        </Field>
        <Field label="Sync" error={errors.schedule}>
          <Select variant="field" bind:value={schedule} options={schedules} label="Sync schedule" />
        </Field>
        {#if writable.length > 1}
          <Field label="Space">
            <Select variant="field" bind:value={space} options={writable.map((s) => ({ id: s.name, label: spaceLabel(s.name), icon: 'folder' }))} label="Space" />
          </Field>
        {/if}
      {/if}
    </Stack>
  </form>
  {#snippet footer()}
    <Button variant="ghost" onclick={() => (open = false)}>Cancel</Button>
    <Button variant="primary" disabled={!kind} loading={busy} onclick={() => submit()}>{kind === 'gmail' || kind === 'drive' ? 'Continue to Google' : 'Connect'}</Button>
  {/snippet}
</Dialog>
