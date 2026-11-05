<script>
  import { Copy } from '@lucide/svelte';
  import { encode } from 'uqr';
  import Section from './Section.svelte';
  import SettingsRow from './SettingsRow.svelte';
  import Button from './Button.svelte';
  import IconButton from './IconButton.svelte';
  import Input from './Input.svelte';
  import { api, said } from '../lib/api.js';

  let on = $state(null);
  let link = $state(null);
  let unlinking = $state(false);
  let code = $state('');
  let error = $state('');
  let busy = $state(false);

  const qr = $derived(link && encode(link.uri, { border: 4 }));
  const dark = $derived(qr && qr.data.flatMap((row, y) => row.flatMap((cell, x) => (cell ? [`M${x} ${y}h1v1h-1z`] : []))).join(''));

  api('/auth/totp')
    .then((out) => (on = out.on))
    .catch((e) => (error = said(e)));

  async function act(work) {
    error = '';
    busy = true;
    try {
      await work();
    } catch (e) {
      error = said(e);
    }
    code = '';
    busy = false;
  }

  const start = () => act(async () => (link = await api('/auth/totp/setup', { method: 'POST' })));

  const post = async (path) => (on = (await api(path, { method: 'POST', body: { code: code.replace(/\D/g, '') } })).on);

  function confirm(event) {
    event.preventDefault();
    act(async () => {
      await post('/auth/totp/enable');
      link = null;
    });
  }

  function unlink(event) {
    event.preventDefault();
    act(async () => {
      await post('/auth/totp/disable');
      unlinking = false;
    });
  }

  function cancel() {
    link = null;
    unlinking = false;
    code = error = '';
  }
</script>

<Section title="Authenticator app">
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if link}
    <svg class="qr" viewBox="0 0 {qr.size} {qr.size}" shape-rendering="crispEdges" role="img" aria-label="QR code"><path d={dark} /></svg>
    <div class="key">
      <span class="value">{link.key}</span>
      <IconButton icon={Copy} label="Copy key" size="sm" onclick={() => navigator.clipboard?.writeText(link.key).catch(() => {})} />
    </div>
    <form class="code" onsubmit={confirm}>
      <Input bind:value={code} aria-label="Code" placeholder="Code" inputmode="numeric" autocomplete="one-time-code" />
      <Button variant="primary" type="submit" loading={busy}>Link</Button>
      <Button variant="ghost" onclick={cancel}>Cancel</Button>
    </form>
  {:else if unlinking}
    <form class="code" onsubmit={unlink}>
      <Input bind:value={code} aria-label="Code" placeholder="Code" inputmode="numeric" autocomplete="one-time-code" />
      <Button variant="primary" type="submit" loading={busy}>Unlink</Button>
      <Button variant="ghost" onclick={cancel}>Cancel</Button>
    </form>
  {:else if on !== null}
    <SettingsRow label={on ? 'Linked' : 'Not linked'}>
      {#if on}
        <Button onclick={() => (unlinking = true)}>Unlink</Button>
      {:else}
        <Button loading={busy} onclick={start}>Link</Button>
      {/if}
    </SettingsRow>
  {/if}
</Section>

<style>
  .qr {
    width: var(--settings-art);
    height: var(--settings-art);
    border-radius: var(--radius-lg);
    background: #fff;
    fill: #000;
  }

  .key {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    max-width: var(--measure-sm);
    min-height: var(--control-xl);
    padding: var(--space-1) var(--space-1) var(--space-1) var(--space-4);
    border-radius: var(--radius-lg);
    background: var(--color-fill-subtle);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-subtle);
  }

  .value {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
    font-size: var(--text-md);
    font-variant-numeric: tabular-nums;
  }

  .code {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    max-width: var(--measure-sm);
  }

  .code :global(.box) {
    flex: 1;
  }

  .error {
    color: var(--color-danger);
    font-size: var(--text-sm);
  }
</style>
