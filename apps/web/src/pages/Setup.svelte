<script lang="ts">
  import { brand } from '../lib/brand.svelte';
  import { AuthFrame, Button, Field, Input, Stack, Notice } from '../lib';
  import { api, ApiError } from '../lib/api';
  import { load } from '../lib/session.svelte';
  import { query } from '../lib/router.svelte';

  const token = query().get('token') ?? '';
  let email = $state('');
  let name = $state('');
  let busy = $state(false);
  let error = $state('');
  let fieldErrors = $state<{ [k: string]: string }>({});

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    error = '';
    fieldErrors = {};
    try {
      await api.setup(token, email.trim(), name.trim());
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.fields.length) {
        for (const f of err.fields) fieldErrors[f.path] = f.message;
      } else error = (err as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<AuthFrame title={`Set up ${brand.name}`} description="Create the first owner. You can invite everyone else once you are in.">
  {#if !token}
    <Notice tone="warning">This page needs the setup link the server printed when it started. Open that link instead.</Notice>
  {:else}
    <form onsubmit={submit} novalidate>
      <Stack gap="4">
        {#if error}<Notice tone="danger">{error}</Notice>{/if}
        <Field label="Your name" id="name">
          <Input id="name" bind:value={name} autocomplete="name" autofocus />
        </Field>
        <Field label="Email" id="email" required error={fieldErrors.email}>
          <Input id="email" type="email" bind:value={email} autocomplete="email" invalid={!!fieldErrors.email} />
        </Field>
        <Button type="submit" variant="primary" size="lg" block loading={busy}>Create owner and sign in</Button>
      </Stack>
    </form>
  {/if}
</AuthFrame>
